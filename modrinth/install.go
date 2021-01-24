package modrinth

import (
	"fmt"
	"os"
	"strings"
	"regexp"
	"time"
	"errors"

	"golang.org/x/mod/semver"
	"github.com/spf13/cobra"
	"github.com/comp500/packwiz/core"
)

var modSiteRegex = regexp.MustCompile("modrinth\\.com\\/mod\\/([^\\/]+)\\/?")

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "install [mod]",
	Short:   "Install a mod from a modrinth URL, slug, ID or search",
	Aliases: []string{"add", "get"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		mcVersion, err := pack.GetMCVersion()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if len(args) == 0 || len(args[0]) == 0 {
            fmt.Println("You must specify a mod.")
            os.Exit(1)
        }


        // If there are more than 1 argument, go straight to searching - URLs/Slugs should not have spaces!
        if len(args) > 1 {
            println(strings.Join(args," "))
            err = installViaSearch(strings.Join(args," "), mcVersion)
            if err != nil {
                fmt.Println(err)
                os.Exit(1)
            }
        }

        //Try interpreting this arg as a mod.
        //Modrinth transparently handles slugs/mod ids in their api. So we don't have to detect whether this is a slug or a mod id.
        var modStr string
        matches := modSiteRegex.FindStringSubmatch(args[0]) //try extracting the mod out of the url
        if matches != nil && len(matches) == 2 {
            modStr = matches[1]
        } else { //This isn't a url. Interpret as a slug/id directly
            modStr = args[0]
        }
        mod, err := fetchMod(modStr)

        if mod.Id != "" { //the mod was found
            fmt.Printf("Found mod %s: '%s'\n", mod.Title, mod.Description)
            err = installMod(mod, pack)
            if err != nil {
                fmt.Println(err)
                os.Exit(1)
            }
            return
        } else { //There wasn't a mod with that exact name, if the user didn't input a url, we should try searching
            if strings.Contains(args[0], "modrinth.com") { //don't bother searching for a url. There shouldn't be a mod with such a name
                err = installViaSearch(args[0], mcVersion)
                if err != nil {
                    fmt.Println(err)
                    os.Exit(1)
                }
            }
        }
	},
}

func installViaSearch(query string, mcVersion string) error {
    _, err := getFirstModIdViaSearch(query, mcVersion)
    if err != nil {
        return err
    }
    return nil
}

func installMod(mod Mod, pack core.Pack) error {
    mcVersion, err := pack.GetMCVersion()
    if err != nil {
        return err;
    }

    versions, err := mod.fetchAllVersions()
    if err != nil {
        return err
    }

    //Tries to find the latest version
    var latestValidVersion Version;
    for _,v := range versions {
        if v.isValid(mcVersion) {
            var semverCompare = semver.Compare(v.Version_number, latestValidVersion.Version_number)
            if semverCompare == 0 {
                //Semver is equal, compare date instead
                vDate, _ := time.Parse(time.RFC3339Nano, v.Date_published)
                latestDate, _ := time.Parse(time.RFC3339Nano, latestValidVersion.Date_published)
                if (vDate.After(latestDate)) {
                    latestValidVersion = v
                }
            } else if semverCompare == 1 {
                latestValidVersion = v
            }
        }
    }

    if latestValidVersion.Id == "" {
        return errors.New("mod not available for this minecraft version")
    }

    return installVersion(mod, latestValidVersion, pack)
}

func installVersion(mod Mod, version Version, pack core.Pack) error {
    var files = version.Files

    if len(files) == 0 {
        return errors.New("version doesn't have any files attached")
    }

    var file = files[0]

    //Install the file
    fmt.Printf("Installing %s from version %s\n", file.Filename, version.Version_number)
    index, err := pack.LoadIndex()
    if err != nil {
        return err
    }

//     updateMap := make(map[string]map[string]interface{})

//     updateMap["modrinth"], err = cfUpdateData{
//         ProjectID: modInfo.ID,
//         FileID:    fileInfo.ID,
//         // TODO: determine update channel
//         ReleaseChannel: "beta",
//     }.ToMap()
//     if err != nil {
//         return err
//     }

    side := mod.getSide()
    if side == "" {
        return errors.New("version is unsupported on both sides")
    }

    algorithm, hash := file.getBestHash()
    if algorithm == "" {
        return errors.New("file doesn't have a hash")
    }

    modMeta := core.Mod{
        Name:     mod.Title,
        FileName: file.Filename,
        Side:     side,
        Download: core.ModDownload{
            URL: file.Url,
            // TODO: murmur2 hashing may be unstable in curse api, calculate the hash manually?
            // TODO: check if the hash is invalid (e.g. 0)
            HashFormat: algorithm,
            Hash:       hash,
        },
        //Update: updateMap,
    }
    var path string
    if mod.Slug != "" {
        path = modMeta.SetMetaName(mod.Slug)
    } else {
        path = modMeta.SetMetaName(mod.Title)
    }

    // If the file already exists, this will overwrite it!!!
    // TODO: Should this be improved?
    // Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
    // VCS anyway.

    format, hash, err := modMeta.Write()
    if err != nil {
        return err
    }

    return index.RefreshFileWithHash(path, format, hash, true)
}

func init() {
	modrinthCmd.AddCommand(installCmd)
}