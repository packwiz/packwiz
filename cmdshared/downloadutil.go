package cmdshared

import (
	"fmt"
	"github.com/packwiz/packwiz/core"
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
		err = os.MkdirAll(filepath.Join(cacheDir, core.DownloadCacheInFolder), 0755)
		if err != nil {
			fmt.Printf("Error creating cache in folder: %v", err)
			os.Exit(1)
		}
		fmt.Printf("Once you have done so, place these files in %s and re-run this command.\n",
			filepath.Join(cacheDir, core.DownloadCacheInFolder))
		os.Exit(1)
	}
}
