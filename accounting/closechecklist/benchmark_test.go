package closechecklist_test

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

// buildBenchmarkTemplate builds a Template with taskCount tasks spread
// across a fixed 10 sections, each task depending on up to depFanIn
// earlier tasks (bounded so the total dependency count matches roughly
// depTotal across the whole template) — section 46's representative
// scale.
func buildBenchmarkTemplate(taskCount, depTotal int) closechecklist.Template {
	sections := make([]closechecklist.SectionDefinition, 10)
	for i := range sections {
		sections[i] = closechecklist.SectionDefinition{SectionCode: fmt.Sprintf("SEC%d", i)}
	}

	tasks := make([]closechecklist.TaskDefinition, taskCount)
	depsPerTask := 0
	if taskCount > 0 {
		depsPerTask = depTotal / taskCount
	}
	for i := 0; i < taskCount; i++ {
		td := closechecklist.TaskDefinition{
			TaskCode:       fmt.Sprintf("t%d", i),
			SectionCode:    fmt.Sprintf("SEC%d", i%10),
			Required:       i%3 != 0,
			Applicability:  closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
			EvidencePolicy: closechecklist.EvidencePolicy{Type: closechecklist.EvidenceAtLeastOne},
			ReviewPolicy:   closechecklist.ReviewPolicy{Type: closechecklist.ReviewPreparerOnly},
			GateRules:      []closechecklist.GateRule{{GateCode: fmt.Sprintf("gate%d", i%20), Require: closechecklist.GateRequirePassOrWarning, Required: i%5 == 0}},
		}
		for d := 1; d <= depsPerTask && i-d >= 0; d++ {
			td.Dependencies = append(td.Dependencies, closechecklist.TaskDependency{DependsOnTaskCode: fmt.Sprintf("t%d", i-d)})
		}
		tasks[i] = td
	}

	return closechecklist.Template{TemplateID: "bench", Version: "1.0", Sections: sections, Tasks: tasks}
}

func buildBenchmarkInstance(taskCount, depTotal, evidenceTotal, signOffTotal, gateTotal int) closechecklist.Instance {
	tmpl := buildBenchmarkTemplate(taskCount, depTotal)

	states := make([]closechecklist.TaskState, taskCount)
	evPerTask := 0
	if taskCount > 0 {
		evPerTask = evidenceTotal / taskCount
	}
	soPerTask := 0
	if taskCount > 0 {
		soPerTask = signOffTotal / taskCount
	}
	for i := 0; i < taskCount; i++ {
		st := closechecklist.TaskState{TaskCode: fmt.Sprintf("t%d", i), Status: closechecklist.TaskCompleted}
		for e := 0; e < evPerTask; e++ {
			st.Evidence = append(st.Evidence, closechecklist.EvidenceRef{EvidenceID: fmt.Sprintf("E%d-%d", i, e)})
		}
		for s := 0; s < soPerTask; s++ {
			st.SignOffs = append(st.SignOffs, closechecklist.SignOff{
				SignOffID: fmt.Sprintf("S%d-%d", i, s), Role: closechecklist.SignOffPreparer,
				ActorRef: fmt.Sprintf("actor%d", s), SignedAt: date("2025-01-31"),
			})
		}
		states[i] = st
	}

	gates := make([]closechecklist.GateFact, 0, gateTotal)
	for g := 0; g < gateTotal; g++ {
		gates = append(gates, closechecklist.GateFact{GateCode: fmt.Sprintf("gate%d", g%20), Status: closechecklist.GatePass})
	}

	return closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates:     states,
		Gates:          gates,
	}
}

// BenchmarkCalculate_Representative uses section 46's laptop-safe
// representative scale: 1,000 tasks, 5,000 dependencies, 2,000 evidence
// records, 2,000 sign-offs, 1,000 gate facts.
func BenchmarkCalculate_Representative(b *testing.B) {
	in := buildBenchmarkInstance(1000, 5000, 2000, 2000, 1000)
	policy := closechecklist.DefaultPolicy()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		closechecklist.Calculate(in, policy)
	}
}

// BenchmarkCalculate_Scaling sweeps 100 -> 500 -> 1,000 tasks (section
// 46), enough to detect an accidental O(N^2) — see bottleneck.go's doc
// comment for the O(N^2) bug this sweep caught during development.
func BenchmarkCalculate_Scaling(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
		n := n
		b.Run(fmt.Sprintf("tasks=%d", n), func(b *testing.B) {
			in := buildBenchmarkInstance(n, n*5, n*2, n*2, n)
			policy := closechecklist.DefaultPolicy()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				closechecklist.Calculate(in, policy)
			}
		})
	}
}

// BenchmarkCalculate_FullScale is gated behind
// CLOSECHECKLIST_FULL_SCALE_BENCH=1 per section 46 — never run by
// default, and never run concurrently with another heavy benchmark
// process.
func BenchmarkCalculate_FullScale(b *testing.B) {
	if on, _ := strconv.ParseBool(os.Getenv("CLOSECHECKLIST_FULL_SCALE_BENCH")); !on {
		b.Skip("set CLOSECHECKLIST_FULL_SCALE_BENCH=1 to run the full-scale benchmark")
	}
	in := buildBenchmarkInstance(20000, 100000, 40000, 40000, 20000)
	policy := closechecklist.DefaultPolicy()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		closechecklist.Calculate(in, policy)
	}
}
