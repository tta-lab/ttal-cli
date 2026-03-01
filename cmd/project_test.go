package cmd

import (
	"context"
	"testing"
	"time"

	"codeberg.org/clawteam/ttal-cli/ent/project"
	"codeberg.org/clawteam/ttal-cli/internal/db"
)

const (
	testNewName = "New Name"
	testNewPath = "/new/path"
)

func setupProjectTest(t *testing.T) {
	t.Helper()
	database = db.NewTestDB(t)
}

func TestProjectModifyAlias(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	proj, err := database.Project.Create().
		SetAlias("old-alias").
		SetName("Test Project").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	_, err = proj.Update().
		SetAlias("new-alias").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project alias: %v", err)
	}

	exists, err := database.Project.Query().
		Where(project.Alias("old-alias")).
		Exist(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}
	if exists {
		t.Error("old alias should not exist after rename")
	}

	updated, err := database.Project.Query().
		Where(project.Alias("new-alias")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project by new alias: %v", err)
	}
	if updated.Alias != "new-alias" {
		t.Errorf("project alias = %v, want new-alias", updated.Alias)
	}
}

func TestProjectModifyName(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Old Name").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	_, err = proj.Update().
		SetName(testNewName).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project name: %v", err)
	}

	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Name != testNewName {
		t.Errorf("project name = %v, want %v", updated.Name, testNewName)
	}
}

func TestProjectModifyDescription(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Test Project").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	_, err = proj.Update().
		SetDescription("Test description").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project description: %v", err)
	}

	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Description != "Test description" {
		t.Errorf("project description = %v, want %v", updated.Description, "Test description")
	}
}

func TestProjectModifyPath(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Test Project").
		SetPath("/old/path").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	_, err = proj.Update().
		SetPath(testNewPath).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project path: %v", err)
	}

	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Path != testNewPath {
		t.Errorf("project path = %v, want %v", updated.Path, testNewPath)
	}
}

func TestProjectModifyMultipleFields(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Old Name").
		SetPath("/old/path").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	_, err = proj.Update().
		SetName(testNewName).
		SetDescription("New description").
		SetPath(testNewPath).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project: %v", err)
	}

	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Name != testNewName {
		t.Errorf("project name = %v, want New Name", updated.Name)
	}
	if updated.Description != "New description" {
		t.Errorf("project description = %v, want New description", updated.Description)
	}
	if updated.Path != testNewPath {
		t.Errorf("project path = %v, want /new/path", updated.Path)
	}
}

func TestProjectArchive(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Test Project").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	if proj.ArchivedAt != nil {
		t.Errorf("new project should not be archived")
	}

	_, err = database.Project.Update().
		Where(project.Alias("test-proj")).
		SetNillableArchivedAt(&[]time.Time{time.Now()}[0]).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to archive project: %v", err)
	}

	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.ArchivedAt == nil {
		t.Errorf("project should be archived")
	}

	_, err = database.Project.Update().
		Where(project.Alias("test-proj")).
		ClearArchivedAt().
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to unarchive project: %v", err)
	}

	updated, err = database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.ArchivedAt != nil {
		t.Errorf("project should not be archived")
	}
}

func TestProjectDelete(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	_, err := database.Project.Create().
		SetAlias("to-delete").
		SetName("Delete Me").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	count, err := database.Project.Delete().
		Where(project.Alias("to-delete")).
		Exec(ctx)
	if err != nil {
		t.Fatalf("failed to delete project: %v", err)
	}
	if count != 1 {
		t.Errorf("deleted %d projects, want 1", count)
	}

	exists, err := database.Project.Query().
		Where(project.Alias("to-delete")).
		Exist(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}
	if exists {
		t.Error("project should not exist after deletion")
	}
}

func TestProjectDeleteNotFound(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	count, err := database.Project.Delete().
		Where(project.Alias("nonexistent")).
		Exec(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("deleted %d projects, want 0", count)
	}
}

func TestProjectListArchivedOnly(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	_, err := database.Project.Create().
		SetAlias("active-proj").
		SetName("Active").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create active project: %v", err)
	}

	now := time.Now()
	_, err = database.Project.Create().
		SetAlias("archived-proj").
		SetName("Archived").
		SetArchivedAt(now).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create archived project: %v", err)
	}

	archived, err := database.Project.Query().
		Where(project.ArchivedAtNotNil()).
		All(ctx)
	if err != nil {
		t.Fatalf("failed to query archived projects: %v", err)
	}
	if len(archived) != 1 {
		t.Errorf("found %d archived projects, want 1", len(archived))
	}
	if len(archived) > 0 && archived[0].Alias != "archived-proj" {
		t.Errorf("archived project alias = %v, want archived-proj", archived[0].Alias)
	}

	active, err := database.Project.Query().
		Where(project.ArchivedAtIsNil()).
		All(ctx)
	if err != nil {
		t.Fatalf("failed to query active projects: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("found %d active projects, want 1", len(active))
	}
	if len(active) > 0 && active[0].Alias != "active-proj" {
		t.Errorf("active project alias = %v, want active-proj", active[0].Alias)
	}
}
