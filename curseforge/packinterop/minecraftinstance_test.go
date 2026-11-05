package packinterop

import (
	"reflect"
	"testing"
)

func TestTwitchInstalledPackMeta_Accessors(t *testing.T) {
	// PackAuthor and PackVersion are intentionally empty for the
	// Twitch (Overwolf) launcher's installed-pack format — that
	// metadata isn't part of the installed-pack JSON. Pinning that.
	m := twitchInstalledPackMeta{NameInternal: "Installed Pack"}

	if got := m.Name(); got != "Installed Pack" {
		t.Errorf("Name = %q, want %q", got, "Installed Pack")
	}

	if got := m.PackAuthor(); got != "" {
		t.Errorf("PackAuthor = %q, want \"\"", got)
	}

	if got := m.PackVersion(); got != "" {
		t.Errorf("PackVersion = %q, want \"\"", got)
	}
}

func TestTwitchInstalledPackMeta_Versions_Forge(t *testing.T) {
	t.Run("MavenVersionString form", func(t *testing.T) {
		m := twitchInstalledPackMeta{MCVersion: "1.20.1"}
		m.Modloader.Name = "forge-something"
		m.Modloader.MavenVersionString = "net.minecraftforge:forge:1.20.1-47.1.0"

		got := m.Versions()
		want := map[string]string{
			"minecraft": "1.20.1",
			"forge":     "47.1.0",
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("falls back to Modloader.Name when MavenVersionString is empty", func(t *testing.T) {
		m := twitchInstalledPackMeta{MCVersion: "1.20.1"}
		m.Modloader.Name = "forge-1.20.1-47.1.0"

		got := m.Versions()
		if got["forge"] != "47.1.0" {
			t.Errorf("forge = %q, want %q", got["forge"], "47.1.0")
		}
	})
}

func TestTwitchInstalledPackMeta_Versions_Fabric(t *testing.T) {
	t.Run("MavenVersionString form", func(t *testing.T) {
		m := twitchInstalledPackMeta{MCVersion: "1.20.1"}
		m.Modloader.Name = "fabric-something"
		m.Modloader.MavenVersionString = "net.fabricmc:fabric-loader:0.15.0"

		got := m.Versions()
		if got["fabric"] != "0.15.0" {
			t.Errorf("fabric = %q, want %q", got["fabric"], "0.15.0")
		}
	})

	t.Run("falls back to Modloader.Name when MavenVersionString is empty", func(t *testing.T) {
		m := twitchInstalledPackMeta{MCVersion: "1.20.1"}
		m.Modloader.Name = "fabric-0.15.0"

		got := m.Versions()
		if got["fabric"] != "0.15.0" {
			t.Errorf("fabric = %q, want %q", got["fabric"], "0.15.0")
		}
	})
}

func TestTwitchInstalledPackMeta_Mods(t *testing.T) {
	m := twitchInstalledPackMeta{}
	m.ModsInternal = []struct {
		ID   uint32 `json:"addonID"`
		File struct {
			ID             uint32 `json:"id"`
			FileNameOnDisk string
		} `json:"installedFile"`
	}{
		{ID: 1},
		{ID: 2},
	}
	// .disabled suffix triggers OptionalDisabled = true.
	m.ModsInternal[0].File.ID = 10
	m.ModsInternal[0].File.FileNameOnDisk = "mod-a.jar"
	m.ModsInternal[1].File.ID = 20
	m.ModsInternal[1].File.FileNameOnDisk = "mod-b.jar.disabled"

	got := m.Mods()

	if len(got) != 2 {
		t.Fatalf("got %d mods, want 2", len(got))
	}

	if got[0].OptionalDisabled {
		t.Errorf("mod-a.jar should NOT be marked OptionalDisabled; got %+v", got[0])
	}

	if !got[1].OptionalDisabled {
		t.Errorf("mod-b.jar.disabled SHOULD be marked OptionalDisabled; got %+v", got[1])
	}

	// Verify ID and FileID flow through.
	if got[0].ProjectID != 1 || got[0].FileID != 10 {
		t.Errorf("got[0] = %+v, want ProjectID=1 FileID=10", got[0])
	}
}
