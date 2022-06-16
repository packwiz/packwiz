package curseforge

import (
	"fmt"
	"github.com/aviddiviner/go-murmur"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// TODO: make all of this less bad and hardcoded

// detectCmd represents the detect command
var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect .jar files in the mods folder (experimental)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		index, err := pack.LoadIndex()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Walk files in the mods folder
		var hashes []uint32
		modPaths := make(map[uint32]string)
		err = filepath.Walk("mods", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".jar") && !strings.HasSuffix(path, ".litemod") {
				// TODO: make this less bad
				return nil
			}
			fmt.Println("Hashing " + path)
			bytes, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			hash := getByteArrayHash(bytes)
			hashes = append(hashes, hash)
			modPaths[hash] = path
			return nil
		})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("Found %d files, submitting...\n", len(hashes))

		res, err := cfDefaultClient.getFingerprintInfo(hashes)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Successfully matched %d files\n", len(res.ExactFingerprints))
		if len(res.PartialMatches) > 0 {
			fmt.Println("The following fingerprints were partial and I don't know what to do!!!")
			for _, v := range res.PartialMatches {
				fmt.Printf("%s (%d)", modPaths[v], v)
			}
		}
		if len(res.UnmatchedFingerprints) > 0 {
			fmt.Printf("Failed to match the following %d files:\n", len(res.UnmatchedFingerprints))
			for _, v := range res.UnmatchedFingerprints {
				fmt.Printf("%s (%d)\n", modPaths[v], v)
			}
		}
		fmt.Println("Installing...")
		for _, v := range res.ExactMatches {
			modInfoData, err := cfDefaultClient.getModInfo(v.ID)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			err = createModFile(modInfoData, v.File, &index, false)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			path, ok := modPaths[v.File.Fingerprint]
			if ok {
				err = os.Remove(path)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
			}
		}
		fmt.Println("Installation done")

		err = index.Refresh()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = index.Write()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = pack.Write()
		if err != nil {
			fmt.Println(err)
			return
		}
	},
}

func init() {
	curseforgeCmd.AddCommand(detectCmd)
}

func getByteArrayHash(bytes []byte) uint32 {
	return murmur.MurmurHash2(computeNormalizedArray(bytes), 1)
}

func computeNormalizedArray(bytes []byte) []byte {
	var newArray []byte
	for _, b := range bytes {
		if !isWhitespaceCharacter(b) {
			newArray = append(newArray, b)
		}
	}
	return newArray
}

func isWhitespaceCharacter(b byte) bool {
	return b == 9 || b == 10 || b == 13 || b == 32
}
