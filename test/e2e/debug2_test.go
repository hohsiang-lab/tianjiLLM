//go:build e2e

package e2e

import (
	"testing"
)

func TestDebugDialog2(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()
	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	// Check the content div's class
	contentDiv := f.Page.Locator("[data-tui-dialog-content][data-dialog-instance='create-model-dialog']")
	cls, err := contentDiv.GetAttribute("class")
	t.Logf("content div class: %s, err: %v", cls, err)

	// Check computed style
	box, err := contentDiv.BoundingBox()
	if err != nil {
		t.Logf("bounding box err: %v", err)
	} else {
		t.Logf("bounding box: x=%.0f y=%.0f w=%.0f h=%.0f", box.X, box.Y, box.Width, box.Height)
	}

	// Check viewport
	viewportSize := f.Page.ViewportSize()
	t.Logf("viewport: %+v", viewportSize)
}
