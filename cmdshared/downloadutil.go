package cmdshared

import (
	"archive/zip"
	"fmt"
	"github.com/packwiz/packwiz/core"
	"io"
	"os"
	"path/filepath"
)

func ListManualDownloads(session core.DownloadSession) {
	manualDownloads := session.GetManualDownloads()
	if len(manualDownloads) > 0 {
		fmt.Printf("Found %v manual downloads; these mods are unable to be downloaded by packwiz (due to API limitations) and must be manually downloaded:\n",
			len(manualDownloads))
		for _, dl := range manualDownloads {
			fmt.Printf("%s (%s) from %s\n", dl.Name, dl.FileName, dl.URL)
		}
		cacheDir, err := core.GetPackwizCache()
		if err != nil {
			fmt.Printf("Error locating cache folder: %v", err)
			os.Exit(1)
		}

		fmt.Printf("Once you have done so, place these files in %s and re-run this command.\n",
			filepath.Join(cacheDir, core.DownloadCacheImportFolder))
		os.Exit(1)
	}
}

func AddToZip(dl core.CompletedDownload, exp *zip.Writer, dir string, indexPath string) bool {
	if dl.Error != nil {
		fmt.Printf("Download of %s (%s) failed: %v\n", dl.Mod.Name, dl.Mod.FileName, dl.Error)
		return false
	}
	for warning := range dl.Warnings {
		fmt.Printf("Warning for %s (%s): %v\n", dl.Mod.Name, dl.Mod.FileName, warning)
	}

	path, err := filepath.Rel(filepath.Dir(indexPath), dl.Mod.GetDestFilePath())
	if err != nil {
		fmt.Printf("Error resolving mod file: %v\n", err)
		return false
	}
	modFile, err := exp.Create(filepath.ToSlash(filepath.Join(dir, path)))
	if err != nil {
		fmt.Printf("Error creating mod file %s: %v\n", path, err)
		return false
	}
	_, err = io.Copy(modFile, dl.File)
	if err != nil {
		fmt.Printf("Error copying file %s: %v\n", path, err)
		return false
	}
	err = dl.File.Close()
	if err != nil {
		fmt.Printf("Error closing file %s: %v\n", path, err)
		return false
	}

	fmt.Printf("%s (%s) added to zip\n", dl.Mod.Name, dl.Mod.FileName)
	return true
}

func PrintDisclaimer(isCf bool) {
	fmt.Println("Disclaimer: you are responsible for ensuring you comply with ALL the licenses, or obtain appropriate permissions, for the files \"added to zip\" below")
	if isCf {
		fmt.Println("Note that mods bundled within a CurseForge pack must be in the Approved Non-CurseForge Mods list")
		fmt.Println("packwiz is currently unable to match metadata between mod sites - if any of these are available from CurseForge you should change them to use CurseForge metadata (e.g. by reinstalling them using the cf commands)")
	} else {
		fmt.Println("packwiz is currently unable to match metadata between mod sites - if any of these are available from Modrinth you should change them to use Modrinth metadata (e.g. by reinstalling them using the mr commands)")
	}
	fmt.Println()
}
