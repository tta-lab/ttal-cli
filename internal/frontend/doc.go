// Package frontend abstracts messaging transport (Telegram, Matrix) behind a
// unified interface. The daemon initialises one Frontend per team and routes
// all human↔agent messaging through it.
//
// Plane: shared
package frontend
