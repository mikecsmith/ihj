//go:build !desktop

package frontend

import "embed"

// Assets is empty when built without the "desktop" tag. Wails populates
// assets at runtime during `wails dev` via its dev server, so the embed
// is only needed for production builds (`wails build -tags desktop`).
var Assets embed.FS
