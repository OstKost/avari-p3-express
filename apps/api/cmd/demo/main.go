// Command demo creates fixtures through the same services used by HTTP.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/OstKost/avari-p3-express/apps/api/internal/database"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/repository/sqlite"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
)

func main() {
	path := flag.String("db", "", "new, nonexistent SQLite file")
	date := flag.String("date", "2026-09-27", "fixture reference date YYYY-MM-DD")
	profile := flag.String("profile", "demo", "demo or smoke fixture")
	flag.Parse()
	if err := seed(*path, *date, *profile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func seed(path, reference, profile string) error {
	if profile != "demo" && profile != "smoke" {
		return fmt.Errorf("invalid profile")
	}
	base, err := time.Parse("2006-01-02", reference)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("--db required")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	f.Close()
	db, err := database.NewConnection(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = database.Migrate(db); err != nil {
		return err
	}
	ctx := context.Background()
	p3 := service.NewP3Service(sqlite.NewP3Repository(db))
	work := service.NewWorkService(sqlite.NewWorkRepository(db))
	manager := domain.WorkActor{ID: "demo-manager", Name: "Демо-менеджер", Manager: true}
	manifest := map[string]any{"schema_version": 2, "fixture_version": 1, "profile": profile, "reference_date": reference, "synthetic": true}
	projects := map[string]string{}
	tasks := map[string]string{}
	expected := map[string]string{}
	steps := map[string]string{}
	names := []string{"Интернет-магазин — разработка", "AI-помощник — пилот", "Миграция CRM — завершённый проект"}
	if profile == "smoke" {
		names = names[:1]
	}
	for i, name := range names {
		due := base.AddDate(0, 0, 14+i*7).Format("2006-01-02")
		p, e := p3.CreateProject(ctx, map[string]any{"name": name, "description": "Демонстрационные данные: сценарии P3.express, SDLC и работы агентов.", "due_date": due})
		if e != nil {
			return e
		}
		projects[[]string{"shop", "ai", "crm"}[i]] = p.ID
		if _, e = p3.Patch(ctx, "project", p.ID, map[string]any{"rag_status": []string{"green", "red", "yellow"}[i], "current_phase": []string{"B", "A", "F"}[i], "archived": i == 2}); e != nil {
			return e
		}
		step := p.Phases[0].Steps[0]
		steps[fmt.Sprintf("%d/A01", i)] = step.ID
		steps[fmt.Sprintf("%d/B01/history", i)] = p.Phases[1].Steps[0].ID
		for j, item := range step.Checklist {
			if j == 0 || i == 2 {
				if _, e = p3.Patch(ctx, "checklist", item.ID, map[string]any{"done": true}); e != nil {
					return e
				}
			}
		}
		if _, e = p3.Create(ctx, "checklist", step.ID, map[string]any{"text": "Демо: документ согласован ответственным"}); e != nil {
			return e
		}
		if _, e = p3.Patch(ctx, "step", step.ID, map[string]any{"status": "in_progress", "due_date": due}); e != nil {
			return e
		}
		for _, input := range []struct {
			kind   string
			fields map[string]any
		}{
			{"link", map[string]any{"title": "Демо: описание результата", "url": "https://example.com/demo", "description": "Условный артефакт, не реальный результат"}},
			{"comment", map[string]any{"text": "Демо: согласовали критерии и следующий этап."}},
		} {
			if _, e = p3.Create(ctx, input.kind, step.ID, input.fields); e != nil {
				return e
			}
		}
		if i == 1 {
			if _, e = p3.Create(ctx, "blocker", p.ID, map[string]any{"title": "Нет тестового набора для оценки модели", "step_id": step.ID}); e != nil {
				return e
			}
		}
		if _, e = p3.Patch(ctx, "sdlc_stage", p.SDLCStages[0].ID, map[string]any{"status": "in_progress", "progress": float64(50)}); e != nil {
			return e
		}
		if i == 0 {
			if _, e = p3.CreateCycle(ctx, p.ID, "B"); e != nil {
				return e
			}
		}
		if _, e = p3.Create(ctx, "action", "", map[string]any{"project_id": p.ID, "step_id": step.ID, "title": "Демо: согласовать план", "priority": "high", "due_date": base.AddDate(0, 0, -1).Format("2006-01-02")}); e != nil {
			return e
		}
		p, e = p3.GetProject(ctx, p.ID)
		if e != nil {
			return e
		}
		steps[fmt.Sprintf("%d/B01", i)] = p.Phases[1].Steps[0].ID
		states := []string{"draft", "ready", "running", "submitted", "done", "returned", "blocked"}
		if profile == "smoke" {
			states = []string{"ready", "submitted"}
		}
		executor := domain.WorkActor{ID: "demo-executor", Name: "Демо-исполнитель", Role: "executor", Projects: []string{p.ID}}
		reviewer := domain.WorkActor{ID: "demo-reviewer", Name: "Демо-QA", Role: "reviewer", Projects: []string{p.ID}}
		for _, state := range states {
			t, e := work.Create(ctx, manager, p.ID, domain.WorkSpec{RequiresIndependentReview: state == "done" || state == "submitted" || state == "returned", Goal: "Демо: задача в состоянии " + state, ExpectedResult: "Условный документ", Criteria: []string{"Демо: документ содержит план"}, Priority: "medium", DueDate: due, StepIDs: []string{step.ID, p.Phases[1].Steps[0].ID}, StageID: p.SDLCStages[0].ID})
			if e != nil {
				return e
			}
			command := func(op string, c service.WorkCommand) error {
				c.Version = t.Version
				var ce error
				actor := executor
				if op == "ready" || op == "review" {
					actor = manager
				}
				t, ce = work.Command(ctx, actor, t.ID, op, c)
				return ce
			}
			if state != "draft" {
				if e = command("ready", service.WorkCommand{}); e != nil {
					return e
				}
			}
			if state != "draft" && state != "ready" {
				if e = command("start", service.WorkCommand{Key: "demo-start"}); e != nil {
					return e
				}
				run := t.Runs[0].ID
				if state == "blocked" {
					if e = command("blocker", service.WorkCommand{RunID: run, Text: "Демо: ожидаем исходные данные"}); e != nil {
						return e
					}
				} else if state != "running" {
					if e = command("result", service.WorkCommand{RunID: run, Key: "demo-result", Result: &domain.TaskResult{Description: "Синтетический результат для демонстрации", Kind: "document", ArtifactVersion: "demo-v1", URLs: []string{"https://example.com/demo"}}}); e != nil {
						return e
					}
					if e = command("submit", service.WorkCommand{RunID: run, Key: "demo-submit", Text: "Демо-отчёт; реальные проверки не выполнялись", Checks: []domain.CriterionCheck{{Criterion: 0, Outcome: "pass", Method: "synthetic demo", Source: "demo", Evidence: "Вымышленное свидетельство для показа UI"}}}); e != nil {
						return e
					}
					if state == "done" {
						t, e = work.AddVerification(ctx, reviewer, run, service.VerificationInput{SubmissionDigest: t.Runs[0].SubmissionDigest, Key: "demo-qa", Kind: "qa", Summary: "Синтетический QA-отчёт, реальные проверки не выполнялись", Checks: []domain.CriterionCheck{{Criterion: 0, Outcome: "pass", Method: "synthetic demo", Source: "demo reviewer", Evidence: "Вымышленное свидетельство для показа UI"}}})
						if e != nil {
							return e
						}
					}
					if state == "done" || state == "returned" {
						decision := "accept"
						if state == "returned" {
							decision = "return"
						}
						if e = command("review", service.WorkCommand{RunID: run, Decision: decision, Text: "Демонстрационное решение менеджера"}); e != nil {
							return e
						}
					}
				}
			}
			key := fmt.Sprintf("%d/%s", i, state)
			tasks[key] = t.ID
			expected[key] = t.Status
		}
	}
	if _, err = p3.Create(ctx, "article", "", map[string]any{"title": "Демо: как посмотреть систему", "category": "P3.express", "summary": "Маршрут по демонстрационным данным", "body": "Откройте проекты, чеклист A1, циклы B, блокеры, SDLC, задачи и результаты. Все отчёты синтетические."}); err != nil {
		return err
	}
	manifest["projects"] = projects
	manifest["tasks"] = tasks
	manifest["expected_task_states"] = expected
	manifest["steps"] = steps
	return json.NewEncoder(os.Stdout).Encode(manifest)
}
