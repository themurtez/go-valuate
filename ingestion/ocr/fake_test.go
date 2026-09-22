package ocr_test

import (
	"context"
	"errors"
	"testing"

	"github.com/themurtez/go-valuate/ingestion/ocr"
)

func TestFakeEngine_ReturnsMappedResult(t *testing.T) {
	engine := &ocr.FakeEngine{
		Pages: map[int]ocr.Result{
			0: {EngineName: "fake", Words: []ocr.Word{{Text: "Revenue", X: 10, Y: 10}}},
		},
	}
	res, err := engine.Recognize(context.Background(), ocr.ImageInput{PageIndex: 0}, ocr.Options{})
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if len(res.Words) != 1 || res.Words[0].Text != "Revenue" {
		t.Errorf("Words = %+v, want one Revenue word", res.Words)
	}
}

func TestFakeEngine_UnmappedPageReturnsEmptyResult(t *testing.T) {
	engine := &ocr.FakeEngine{}
	res, err := engine.Recognize(context.Background(), ocr.ImageInput{PageIndex: 5}, ocr.Options{})
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if len(res.Words) != 0 {
		t.Errorf("Words = %+v, want empty", res.Words)
	}
}

func TestFakeEngine_ErrOnPage(t *testing.T) {
	wantErr := &ocr.Error{Code: ocr.ErrCodeEngineFailed, Message: "boom"}
	engine := &ocr.FakeEngine{
		ErrOnPage: map[int]error{2: wantErr},
	}
	_, err := engine.Recognize(context.Background(), ocr.ImageInput{PageIndex: 2}, ocr.Options{})
	if !errors.Is(err, error(wantErr)) && err != wantErr {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}

func TestFakeEngine_RespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine := &ocr.FakeEngine{}
	_, err := engine.Recognize(ctx, ocr.ImageInput{PageIndex: 0}, ocr.Options{})
	if err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
}

func TestFakeEngine_RecordsCalls(t *testing.T) {
	engine := &ocr.FakeEngine{}
	_, _ = engine.Recognize(context.Background(), ocr.ImageInput{PageIndex: 3}, ocr.Options{})
	_, _ = engine.Recognize(context.Background(), ocr.ImageInput{PageIndex: 7}, ocr.Options{})
	if len(engine.Calls) != 2 || engine.Calls[0] != 3 || engine.Calls[1] != 7 {
		t.Errorf("Calls = %v, want [3 7]", engine.Calls)
	}
}

func TestError_MessageFormatting(t *testing.T) {
	e := &ocr.Error{Code: ocr.ErrCodeEngineUnavailable, Message: "not installed"}
	if got := e.Error(); got != "ocr: OCR_ENGINE_UNAVAILABLE: not installed" {
		t.Errorf("Error() = %q", got)
	}
	e2 := &ocr.Error{Code: ocr.ErrCodeEngineFailed, Message: "bad exit", Detail: "exit status 1"}
	if got := e2.Error(); got != "ocr: OCR_ENGINE_FAILED: bad exit (exit status 1)" {
		t.Errorf("Error() = %q", got)
	}
}
