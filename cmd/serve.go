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

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var refreshMutex sync.RWMutex

//go:embed serve-templates/index.html
var indexPage string

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Run a local development server",
	Long:    `Run a local HTTP server for development, automatically refreshing the index when it is queried`,
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
			indexPath := filepath.Join(filepath.Dir(viper.GetString("pack-file")), filepath.FromSlash(pack.Index.File))
			indexDir := filepath.Dir(indexPath)

			t, err := template.New("index-page").Parse(indexPage)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			indexPageBuf := new(bytes.Buffer)
			t.Execute(indexPageBuf, struct{ Port string }{Port: port})

			http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/" {
					_, _ = w.Write(indexPageBuf.Bytes())
					return
				}

				urlPath := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(req.URL.Path, "/")), "/")
				indexRelPath, err := filepath.Rel(indexDir, filepath.FromSlash(urlPath))
				if err != nil {
					fmt.Println(err)
					return
				}
				indexRelPathSlash := path.Clean(filepath.ToSlash(indexRelPath))
				var destPath string

				found := false
				if urlPath == filepath.ToSlash(indexPath) {
					found = true
					destPath = indexPath
					// Must be done here, to ensure all paths gain the lock at some point
					refreshMutex.RLock()
				} else if urlPath == filepath.ToSlash(viper.GetString("pack-file")) {
					found = true
					if viper.GetBool("serve.refresh") {
						// Get write lock, to do a refresh
						refreshMutex.Lock()
						// Reload pack and index (might have changed on disk)
						pack, err = core.LoadPack()
						if err != nil {
							fmt.Println(err)
							return
						}
						index, err = pack.LoadIndex()
						if err != nil {
							fmt.Println(err)
							return
						}
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
						fmt.Println("Index refreshed!")

						// Downgrade to a read lock
						refreshMutex.Unlock()
					}
					refreshMutex.RLock()
					destPath = viper.GetString("pack-file")
				} else {
					refreshMutex.RLock()
					// Only allow indexed files
					for _, v := range index.Files {
						if indexRelPathSlash == v.File {
							found = true
							break
						}
					}
					if found {
						destPath = filepath.FromSlash(urlPath)
					}
				}
				defer refreshMutex.RUnlock()
				if found {
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
				} else {
					fmt.Printf("File not found: %s\n", destPath)
					w.WriteHeader(404)
					_, _ = w.Write([]byte("File not found"))
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

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntP("port", "p", 8080, "The port to run the server on")
	_ = viper.BindPFlag("serve.port", serveCmd.Flags().Lookup("port"))
	serveCmd.Flags().BoolP("refresh", "r", true, "Automatically refresh the index file")
	_ = viper.BindPFlag("serve.refresh", serveCmd.Flags().Lookup("refresh"))
	serveCmd.Flags().Bool("basic", false, "Disable refreshing and allow all files in the directory, rather than just files listed in the index")
	_ = viper.BindPFlag("serve.basic", serveCmd.Flags().Lookup("basic"))
}
