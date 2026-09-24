package closechecklist_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

// FuzzCalculate_Template exercises template validation, dependency-graph
// handling, and readiness calculation against arbitrarily mutated
// task/dependency shapes — section 48. It never expects a specific
// result, only: no panic, no infinite loop (bounded via a small,
// finite task count so any hang is caught by the test timeout), and
// READY_TO_CLOSE reachable only when it should be.
func FuzzCalculate_Template(f *testing.F) {
	f.Add(2, 0, 1, true, false)
	f.Add(3, 2, 5, false, true)
	f.Add(5, 4, 10, true, true)
	f.Add(1, 0, 0, false, false)

	f.Fuzz(func(t *testing.T, taskCount, depSeed, statusSeed int, required, allowSkip bool) {
		if taskCount < 0 {
			taskCount = -taskCount
		}
		if taskCount > 50 {
			taskCount = taskCount % 50
		}
		if taskCount == 0 {
			taskCount = 1
		}

		tasks := make([]closechecklist.TaskDefinition, taskCount)
		for i := 0; i < taskCount; i++ {
			td := closechecklist.TaskDefinition{
				TaskCode:      taskCodeFor(i),
				SectionCode:   "A",
				Required:      required,
				AllowSkip:     allowSkip,
				Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
			}
			// depSeed picks a pseudo-random earlier (or, deliberately, a
			// later/self) task to depend on, to exercise unknown/self/
			// forward-reference/cycle validation paths too.
			if depSeed != 0 {
				depIdx := ((depSeed + i*7) % (taskCount + 1))
				if depIdx >= 0 && depIdx < taskCount {
					td.Dependencies = []closechecklist.TaskDependency{{DependsOnTaskCode: taskCodeFor(depIdx)}}
				}
			}
			tasks[i] = td
		}

		tmpl := closechecklist.Template{
			TemplateID: "fuzz", Version: "1.0",
			Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
			Tasks:    tasks,
		}

		var states []closechecklist.TaskState
		statuses := []closechecklist.TaskStatus{
			closechecklist.TaskNotStarted, closechecklist.TaskInProgress, closechecklist.TaskCompleted,
			closechecklist.TaskBlocked, closechecklist.TaskSkipped, closechecklist.TaskNotApplicable,
		}
		for i := 0; i < taskCount; i++ {
			idx := (statusSeed + i) % len(statuses)
			if idx < 0 {
				idx = -idx
			}
			states = append(states, closechecklist.TaskState{TaskCode: taskCodeFor(i), Status: statuses[idx]})
		}

		in := closechecklist.Instance{
			Template:       tmpl,
			Period:         closechecklist.Period{PeriodID: "P", StartDate: fixedDate, EndDate: fixedDate.AddDate(0, 1, 0), TargetCloseDate: fixedDate.AddDate(0, 1, 5)},
			EvaluationDate: fixedDate.AddDate(0, 1, 3),
			TaskStates:     states,
		}

		res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

		// Invariant: SatisfiedRequiredTaskCount <= RequiredTaskCount.
		if res.Completion.SatisfiedRequiredTaskCount > res.Completion.RequiredTaskCount {
			t.Fatalf("satisfied required (%d) > required (%d)", res.Completion.SatisfiedRequiredTaskCount, res.Completion.RequiredTaskCount)
		}
		// Invariant: READY_TO_CLOSE implies every required applicable
		// task is satisfied and no effective blocker remains.
		if res.Readiness == closechecklist.ChecklistReadyToClose {
			for _, tr := range res.Tasks {
				if tr.Applicability == closechecklist.ApplicableYes && tr.Required && !tr.Satisfied {
					t.Fatalf("READY_TO_CLOSE but required applicable task %s is not satisfied", tr.TaskCode)
				}
				if tr.EffectiveBlocking {
					t.Fatalf("READY_TO_CLOSE but task %s has an effective blocker", tr.TaskCode)
				}
			}
		}
	})
}

var fixedDate = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func taskCodeFor(i int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	return "t" + string(letters[i%len(letters)]) + string(rune('0'+i/len(letters)))
}

// FuzzEvaluateReview exercises evidence/sign-off/exception validation
// paths indirectly through Calculate with randomized evidence/sign-off
// shapes — section 48.
func FuzzCalculate_EvidenceAndSignOff(f *testing.F) {
	f.Add(1, 2, true)
	f.Add(0, 0, false)
	f.Add(5, 5, true)

	f.Fuzz(func(t *testing.T, evidenceCount, signOffCount int, distinct bool) {
		if evidenceCount < 0 {
			evidenceCount = -evidenceCount
		}
		if evidenceCount > 20 {
			evidenceCount = evidenceCount % 20
		}
		if signOffCount < 0 {
			signOffCount = -signOffCount
		}
		if signOffCount > 20 {
			signOffCount = signOffCount % 20
		}

		tmpl := closechecklist.Template{
			TemplateID: "fuzz2", Version: "1.0",
			Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
			Tasks: []closechecklist.TaskDefinition{{
				TaskCode: "t1", SectionCode: "A", Required: true,
				Applicability:  closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				EvidencePolicy: closechecklist.EvidencePolicy{Type: closechecklist.EvidenceMinCount, MinCount: 3},
				ReviewPolicy:   closechecklist.ReviewPolicy{Type: closechecklist.ReviewPreparerAndReviewer, RequireDistinctActors: distinct},
			}},
		}

		var evidence []closechecklist.EvidenceRef
		for i := 0; i < evidenceCount; i++ {
			evidence = append(evidence, closechecklist.EvidenceRef{EvidenceID: taskCodeFor(i)})
		}
		var signOffs []closechecklist.SignOff
		roles := []closechecklist.SignOffRole{closechecklist.SignOffPreparer, closechecklist.SignOffReviewer}
		for i := 0; i < signOffCount; i++ {
			actor := "actor1"
			if !distinct {
				actor = taskCodeFor(i)
			}
			signOffs = append(signOffs, closechecklist.SignOff{
				SignOffID: taskCodeFor(i), Role: roles[i%2], ActorRef: actor, SignedAt: fixedDate,
			})
		}

		in := closechecklist.Instance{
			Template:       tmpl,
			Period:         closechecklist.Period{PeriodID: "P", StartDate: fixedDate, EndDate: fixedDate.AddDate(0, 1, 0), TargetCloseDate: fixedDate.AddDate(0, 1, 5)},
			EvaluationDate: fixedDate.AddDate(0, 1, 3),
			TaskStates: []closechecklist.TaskState{
				{TaskCode: "t1", Status: closechecklist.TaskCompleted, Evidence: evidence, SignOffs: signOffs},
			},
		}

		res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
		if len(res.Tasks) != 1 {
			t.Fatalf("expected exactly 1 task result, got %d", len(res.Tasks))
		}
	})
}
