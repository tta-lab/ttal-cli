package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/guion-opensource/ttal-cli/ent/project"
	"github.com/guion-opensource/ttal-cli/ent/tag"
	"github.com/guion-opensource/ttal-cli/internal/db"
)

func setupProjectTest(t *testing.T) {
	t.Helper()
	database = db.NewTestDB(t)
}

func TestProjectModifyName(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	// Create project
	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Old Name").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Modify name
	_, err = proj.Update().
		SetName("New Name").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project name: %v", err)
	}

	// Verify
	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Name != "New Name" {
		t.Errorf("project name = %v, want %v", updated.Name, "New Name")
	}
}

func TestProjectModifyDescription(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	// Create project without description
	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Test Project").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Add description
	_, err = proj.Update().
		SetDescription("Test description").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project description: %v", err)
	}

	// Verify
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

	// Create project
	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Test Project").
		SetPath("/old/path").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Modify path
	_, err = proj.Update().
		SetPath("/new/path").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project path: %v", err)
	}

	// Verify
	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Path != "/new/path" {
		t.Errorf("project path = %v, want %v", updated.Path, "/new/path")
	}
}

func TestProjectModifyRepo(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	// Create project
	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Test Project").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Set repo fields
	_, err = proj.Update().
		SetRepo("owner/repo").
		SetRepoType(project.RepoTypeForgejo).
		SetOwner("owner").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project repo: %v", err)
	}

	// Verify
	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Repo != "owner/repo" {
		t.Errorf("project repo = %v, want %v", updated.Repo, "owner/repo")
	}
	if updated.RepoType != project.RepoTypeForgejo {
		t.Errorf("project repo_type = %v, want %v", updated.RepoType, project.RepoTypeForgejo)
	}
	if updated.Owner != "owner" {
		t.Errorf("project owner = %v, want %v", updated.Owner, "owner")
	}
}

func TestProjectModifyRepoType(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		repoType project.RepoType
	}{
		{"forgejo", project.RepoTypeForgejo},
		{"github", project.RepoTypeGithub},
		{"codeberg", project.RepoTypeCodeberg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create project
			proj, err := database.Project.Create().
				SetAlias("test-" + tt.name).
				SetName("Test Project").
				Save(ctx)
			if err != nil {
				t.Fatalf("failed to create project: %v", err)
			}

			// Set repo type
			_, err = proj.Update().
				SetRepoType(tt.repoType).
				Save(ctx)
			if err != nil {
				t.Fatalf("failed to update repo type: %v", err)
			}

			// Verify
			updated, err := database.Project.Query().
				Where(project.Alias("test-" + tt.name)).
				Only(ctx)
			if err != nil {
				t.Fatalf("failed to query project: %v", err)
			}

			if updated.RepoType != tt.repoType {
				t.Errorf("project repo_type = %v, want %v", updated.RepoType, tt.repoType)
			}
		})
	}
}

func TestProjectModifyTags(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	// Create tags
	tag1, err := database.Tag.Create().SetName("backend").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create tag1: %v", err)
	}
	tag2, err := database.Tag.Create().SetName("frontend").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create tag2: %v", err)
	}

	// Create project with tag1
	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Test Project").
		AddTags(tag1).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Add tag2
	_, err = proj.Update().
		AddTags(tag2).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to add tag: %v", err)
	}

	// Verify project has both tags
	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		WithTags().
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if len(updated.Edges.Tags) != 2 {
		t.Errorf("project has %d tags, want 2", len(updated.Edges.Tags))
	}

	// Remove tag1
	_, err = updated.Update().
		RemoveTags(tag1).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to remove tag: %v", err)
	}

	// Verify project only has tag2
	updated, err = database.Project.Query().
		Where(project.Alias("test-proj")).
		WithTags().
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if len(updated.Edges.Tags) != 1 {
		t.Errorf("project has %d tags, want 1", len(updated.Edges.Tags))
	}
	if updated.Edges.Tags[0].Name != "frontend" {
		t.Errorf("project tag = %v, want frontend", updated.Edges.Tags[0].Name)
	}
}

func TestProjectModifyMultipleFields(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	// Create project
	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Old Name").
		SetPath("/old/path").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Modify multiple fields at once
	_, err = proj.Update().
		SetName("New Name").
		SetDescription("New description").
		SetPath("/new/path").
		SetRepo("owner/repo").
		SetRepoType(project.RepoTypeGithub).
		SetOwner("newowner").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project: %v", err)
	}

	// Verify all fields
	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Name != "New Name" {
		t.Errorf("project name = %v, want New Name", updated.Name)
	}
	if updated.Description != "New description" {
		t.Errorf("project description = %v, want New description", updated.Description)
	}
	if updated.Path != "/new/path" {
		t.Errorf("project path = %v, want /new/path", updated.Path)
	}
	if updated.Repo != "owner/repo" {
		t.Errorf("project repo = %v, want owner/repo", updated.Repo)
	}
	if updated.RepoType != project.RepoTypeGithub {
		t.Errorf("project repo_type = %v, want github", updated.RepoType)
	}
	if updated.Owner != "newowner" {
		t.Errorf("project owner = %v, want newowner", updated.Owner)
	}
}

func TestProjectModifyCombined(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	// Create tags
	oldTag, err := database.Tag.Create().SetName("old").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create old tag: %v", err)
	}
	newTag, err := database.Tag.Create().SetName("new").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create new tag: %v", err)
	}

	// Create project
	proj, err := database.Project.Create().
		SetAlias("test-proj").
		SetName("Old Name").
		SetPath("/old/path").
		AddTags(oldTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Modify fields and tags in one operation
	_, err = proj.Update().
		SetName("New Name").
		SetPath("/new/path").
		SetDescription("New description").
		AddTags(newTag).
		RemoveTags(oldTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update project: %v", err)
	}

	// Verify
	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		WithTags().
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.Name != "New Name" {
		t.Errorf("project name = %v, want New Name", updated.Name)
	}
	if updated.Path != "/new/path" {
		t.Errorf("project path = %v, want /new/path", updated.Path)
	}
	if updated.Description != "New description" {
		t.Errorf("project description = %v, want New description", updated.Description)
	}
	if len(updated.Edges.Tags) != 1 {
		t.Errorf("project has %d tags, want 1", len(updated.Edges.Tags))
	}
	if updated.Edges.Tags[0].Name != "new" {
		t.Errorf("project tag = %v, want new", updated.Edges.Tags[0].Name)
	}
}

func TestProjectArchive(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	// Create project
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

	// Archive project
	_, err = database.Project.Update().
		Where(project.Alias("test-proj")).
		SetNillableArchivedAt(&[]time.Time{time.Now()}[0]).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to archive project: %v", err)
	}

	// Verify archived
	updated, err := database.Project.Query().
		Where(project.Alias("test-proj")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if updated.ArchivedAt == nil {
		t.Errorf("project should be archived")
	}

	// Unarchive
	_, err = database.Project.Update().
		Where(project.Alias("test-proj")).
		ClearArchivedAt().
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to unarchive project: %v", err)
	}

	// Verify unarchived
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

func TestProjectQueryWithTags(t *testing.T) {
	setupProjectTest(t)
	ctx := context.Background()

	// Create tags
	backendTag, err := database.Tag.Create().SetName("backend").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create backend tag: %v", err)
	}
	frontendTag, err := database.Tag.Create().SetName("frontend").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create frontend tag: %v", err)
	}

	// Create projects
	_, err = database.Project.Create().
		SetAlias("proj1").
		SetName("Project 1").
		AddTags(backendTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create proj1: %v", err)
	}

	_, err = database.Project.Create().
		SetAlias("proj2").
		SetName("Project 2").
		AddTags(frontendTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create proj2: %v", err)
	}

	_, err = database.Project.Create().
		SetAlias("proj3").
		SetName("Project 3").
		AddTags(backendTag, frontendTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create proj3: %v", err)
	}

	// Query projects with backend tag
	backendProjects, err := database.Project.Query().
		Where(project.HasTagsWith(tag.Name("backend"))).
		All(ctx)
	if err != nil {
		t.Fatalf("failed to query backend projects: %v", err)
	}

	if len(backendProjects) != 2 {
		t.Errorf("found %d backend projects, want 2", len(backendProjects))
	}

	// Query projects with frontend tag
	frontendProjects, err := database.Project.Query().
		Where(project.HasTagsWith(tag.Name("frontend"))).
		All(ctx)
	if err != nil {
		t.Fatalf("failed to query frontend projects: %v", err)
	}

	if len(frontendProjects) != 2 {
		t.Errorf("found %d frontend projects, want 2", len(frontendProjects))
	}
}
