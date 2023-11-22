package cmd

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var refreshMutex sync.RWMutex

//go:embed serve-templates/index.html
var indexPage string

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Run a local development server.",
	Long:    `Run a local HTTP server for development, automatically refreshing the index when it is queried.`,
	Aliases: []string{"server"},
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		port := strconv.Itoa(viper.GetInt("serve.port"))

		if viper.GetBool("serve.basic") {
			http.Handle("/", http.FileServer(http.Dir(".")))
		} else {
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
			packServeDir := filepath.Dir(viper.GetString("pack-file"))
			packFileName := filepath.Base(viper.GetString("pack-file"))

			t, err := template.New("index-page").Parse(indexPage)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			indexPageBuf := new(bytes.Buffer)
			err = t.Execute(indexPageBuf, struct{ Port string }{Port: port})
			if err != nil {
				panic(fmt.Errorf("failed to compile index page template: %w", err))
			}

			// Force-disable no-internal-hashes mode (equiv to --build flag in refresh) for serving over HTTP
			if viper.GetBool("no-internal-hashes") {
				fmt.Println("Note: no-internal-hashes mode is set; still writing hashes for use with packwiz-installer - run packwiz refresh to remove them.")
				viper.Set("no-internal-hashes", false)
			}

			http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/" {
					_, _ = w.Write(indexPageBuf.Bytes())
					return
				}

				// Relative to pack.toml
				urlPath := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(req.URL.Path, "/")), "/")
				// Convert to absolute
				destPath := filepath.Join(packServeDir, filepath.FromSlash(urlPath))
				// Relativisation needs to be done using filepath, as path doesn't have Rel!
				// (now using index util function)
				// Relative to index.toml ("pack root")
				indexRelPath, err := index.RelIndexPath(destPath)
				if err != nil {
					fmt.Println("Failed to parse path", err)
					return
				}

				if urlPath == path.Clean(pack.Index.File) {
					// Must be done here, to ensure all paths gain the lock at some point
					refreshMutex.RLock()
				} else if urlPath == packFileName { // Only need to compare name - already relative to pack.toml
					if viper.GetBool("serve.refresh") {
						// Get write lock, to do a refresh
						refreshMutex.Lock()
						// Reload pack and index (might have changed on disk)
						err = doServeRefresh(&pack, &index)
						if err != nil {
							fmt.Println("Failed to refresh pack", err)
							return
						}

						// Downgrade to a read lock
						refreshMutex.Unlock()
					}
					refreshMutex.RLock()
				} else {
					refreshMutex.RLock()
					// Only allow indexed files
					if _, found := index.Files[indexRelPath]; !found {
						fmt.Printf("File not found: %s\n", destPath)
						refreshMutex.RUnlock()
						w.WriteHeader(404)
						_, _ = w.Write([]byte("File not found"))
						return
					}
				}
				defer refreshMutex.RUnlock()

				f, err := os.Open(destPath)
				if err != nil {
					fmt.Printf("Error reading file \"%s\": %s\n", destPath, err)
					w.WriteHeader(404)
					_, _ = w.Write([]byte("File not found"))
					return
				}
				_, err = io.Copy(w, f)
				err2 := f.Close()
				if err == nil {
					err = err2
				}
				if err != nil {
					fmt.Printf("Error reading file \"%s\": %s\n", destPath, err)
					w.WriteHeader(500)
					_, _ = w.Write([]byte("Failed to read file"))
					return
				}
			})
		}

		fmt.Println("Running on port " + port)
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			fmt.Printf("Error running server: %s\n", err)
			os.Exit(1)
		}
	},
}

func doServeRefresh(pack *core.Pack, index *core.Index) error {
	var err error
	*pack, err = core.LoadPack()
	if err != nil {
		return err
	}
	*index, err = pack.LoadIndex()
	if err != nil {
		return err
	}
	err = index.Refresh()
	if err != nil {
		return err
	}
	err = index.Write()
	if err != nil {
		return err
	}
	err = pack.UpdateIndexHash()
	if err != nil {
		return err
	}
	err = pack.Write()
	if err != nil {
		return err
	}
	fmt.Println("Index refreshed!")

	return nil
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntP("port", "p", 8080, "The port to run the server on")
	_ = viper.BindPFlag("serve.port", serveCmd.Flags().Lookup("port"))
	serveCmd.Flags().BoolP("refresh", "r", true, "Automatically refresh the index file")
	_ = viper.BindPFlag("serve.refresh", serveCmd.Flags().Lookup("refresh"))
	serveCmd.Flags().Bool("basic", false, "Disable refreshing and allow all files in the directory, rather than just files listed in the index")
	_ = viper.BindPFlag("serve.basic", serveCmd.Flags().Lookup("basic"))
}
