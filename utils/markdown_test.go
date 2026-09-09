package utils

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestDisableTag(t *testing.T) {
	t.Run("single leaf command", func(t *testing.T) {
		c := &cobra.Command{Use: "root"}

		disableTag(c)

		if !c.DisableAutoGenTag {
			t.Error("DisableAutoGenTag was not set on a leaf command")
		}
	})

	t.Run("propagates to children and grandchildren", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		child := &cobra.Command{Use: "child"}
		grandchild := &cobra.Command{Use: "grandchild"}

		root.AddCommand(child)
		child.AddCommand(grandchild)

		disableTag(root)

		for _, c := range []*cobra.Command{root, child, grandchild} {
			if !c.DisableAutoGenTag {
				t.Errorf("DisableAutoGenTag was not set on command %q", c.Use)
			}
		}
	})

	t.Run("propagates across multiple siblings", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		a := &cobra.Command{Use: "a"}
		b := &cobra.Command{Use: "b"}
		c := &cobra.Command{Use: "c"}

		root.AddCommand(a, b, c)

		disableTag(root)

		for _, cmd := range []*cobra.Command{root, a, b, c} {
			if !cmd.DisableAutoGenTag {
				t.Errorf("DisableAutoGenTag was not set on command %q", cmd.Use)
			}
		}
	})
}
