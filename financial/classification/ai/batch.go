package ai

import "context"

// DefaultMaxBatchSize is Policy.MaxBatchSize's default when zero.
const DefaultMaxBatchSize = 20

// BatchClassifier is an OPTIONAL capability a Classifier implementation may
// additionally satisfy to classify several rows in one provider call for
// cost/latency efficiency (see the README's batching section).
// ClassifyBatchWithFallback type-asserts for this interface and uses it
// when available, falling back to sequential Classifier.Classify calls
// otherwise — a caller/adapter never has to implement batching just to
// satisfy Classifier.
//
// A BatchClassifier implementation MUST uphold every guarantee
// ClassifyBatchWithFallback documents regardless of provider-side batching
// details: result[i] corresponds to reqs[i] (input order preserved), one
// malformed/failed row's response must not prevent any other row's
// response from being returned (return a zero Response for that index and
// report the problem via the corresponding error, never drop the index or
// reorder), and the returned slice is always exactly len(reqs) long.
type BatchClassifier interface {
	Classifier
	// ClassifyBatch classifies every entry in reqs, returning a
	// same-length results slice and a same-length errs slice (errs[i] is
	// nil when results[i] is valid). Must respect ctx cancellation.
	ClassifyBatch(ctx context.Context, reqs []Request) (results []Response, errs []error)
}

// classifyMany dispatches reqs to classifier, using its BatchClassifier
// capability (split into Policy.MaxBatchSize-sized chunks) when available,
// or sequential Classify calls otherwise. Always returns a
// len(reqs)-long results/errs pair with reqs[i] <-> results[i]/errs[i],
// regardless of which path was taken — this is the single implementation
// both ClassifyWithFallback (a length-1 call) and
// ClassifyBatchWithFallback route through, so the two entry points can
// never drift in their ordering/error-isolation guarantees.
func classifyMany(ctx context.Context, classifier Classifier, reqs []Request, maxBatchSize int) ([]Response, []error) {
	results := make([]Response, len(reqs))
	errs := make([]error, len(reqs))

	batcher, ok := classifier.(BatchClassifier)
	if !ok {
		for i, req := range reqs {
			results[i], errs[i] = classifier.Classify(ctx, req)
		}
		return results, errs
	}

	if maxBatchSize <= 0 {
		maxBatchSize = DefaultMaxBatchSize
	}
	for start := 0; start < len(reqs); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(reqs) {
			end = len(reqs)
		}
		chunkResults, chunkErrs := batcher.ClassifyBatch(ctx, reqs[start:end])
		// A misbehaving BatchClassifier that returns the wrong length must
		// never corrupt unrelated rows' indices — treat every row in this
		// chunk as a provider error instead of trusting a misaligned slice.
		if len(chunkResults) != end-start || len(chunkErrs) != end-start {
			for i := start; i < end; i++ {
				errs[i] = errMalformedBatchResult
			}
			continue
		}
		copy(results[start:end], chunkResults)
		copy(errs[start:end], chunkErrs)
	}
	return results, errs
}

var errMalformedBatchResult = &batchLengthError{}

// batchLengthError reports a BatchClassifier that returned a results/errs
// slice of the wrong length for its input chunk — a provider-adapter bug,
// never a legitimate provider response, so it is always treated as
// IssueProviderError (see classifyProviderError).
type batchLengthError struct{}

func (*batchLengthError) Error() string {
	return "ai: BatchClassifier returned a results/errs slice of unexpected length"
}
