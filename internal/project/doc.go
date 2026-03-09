// Package project manages the TOML-backed project registry for ttal.
//
// Projects are stored in ~/.config/ttal/projects.toml (or a per-team variant) as
// top-level alias sections with name and path fields; archived projects live under
// [archived]. The Store type provides CRUD and archive/unarchive operations with
// atomic writes, while resolve.go resolves project paths from taskwarrior project strings.
//
// Plane: shared
package project
