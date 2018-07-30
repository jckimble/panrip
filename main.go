package main

import (
	"fmt"
	"github.com/bogem/id3v2"
	"github.com/cellofellow/gopiano"
	"github.com/cellofellow/gopiano/responses"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var RootCmd = &cobra.Command{
	Use:   "panrip",
	Short: "Download Music From Pandora",
	Run: func(cmd *cobra.Command, args []string) {
		panrip := &panrip{
			Email:    viper.Get("email").(string),
			Password: viper.Get("password").(string),
			Out:      viper.Get("out").(string),
		}
		switch t := viper.Get("verbose").(type) {
		case string:
			if t == "true" || t == "yes" || t == "y" {
				panrip.Verbose = true
			}
		case bool:
			panrip.Verbose = t
		}
		err := panrip.login()
		if err != nil {
			fmt.Printf("Login Error: %s\n", err)
			os.Exit(1)
		}
		panrip.run()
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if viper.Get("email").(string) == "" || viper.Get("password").(string) == "" {
			return fmt.Errorf("Email and Password must be set by either env, config file or command line")
		}
		return nil
	},
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file")

	RootCmd.Flags().BoolP("verbose", "v", false, "Output Each Track")
	RootCmd.Flags().StringP("email", "e", "", "Email Address For Pandora Account")
	RootCmd.Flags().StringP("password", "p", "", "Password For Pandora Account")
	RootCmd.Flags().StringP("out", "o", "download", "Out Directory of Downloads")
	viper.BindPFlag("password", RootCmd.Flags().Lookup("password"))
	viper.BindPFlag("email", RootCmd.Flags().Lookup("email"))
	viper.BindPFlag("verbose", RootCmd.Flags().Lookup("verbose"))
	viper.BindPFlag("out", RootCmd.Flags().Lookup("out"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
	viper.AutomaticEnv()
	viper.ReadInConfig()
}

type panrip struct {
	Email    string
	Password string
	Out      string
	Verbose  bool

	pandora *gopiano.Client
	stop    chan os.Signal
}

func (p *panrip) login() error {
	pandora, err := gopiano.NewClient(gopiano.AndroidClient)
	if err != nil {
		return err
	}
	_, err = pandora.AuthPartnerLogin()
	if err != nil {
		return err
	}
	_, err = pandora.AuthUserLogin(p.Email, p.Password)
	if err != nil {
		return err
	}
	p.pandora = pandora
	return nil
}
func (p *panrip) recover(err error) {
	switch t := err.(type) {
	case responses.ErrorResponse:
		if t.Message == "INVALID_AUTH_TOKEN" {
			err := p.login()
			if err != nil {
				fmt.Printf("Login Error: %s\n", err)
				os.Exit(1)
			}
		}
	}
}
func (p *panrip) run() {
	stations, err := p.pandora.UserGetStationList(false)
	if err != nil {
		fmt.Printf("Unexpected error(StationList): %s\n", err)
		os.Exit(1)
	}
	p.stop = make(chan os.Signal)
	signal.Notify(p.stop, syscall.SIGTERM)
	signal.Notify(p.stop, syscall.SIGINT)
	for _, item := range stations.Result.Stations {
		if p.Verbose {
			fmt.Printf("Downloading from %s\n", item.StationName)
		}
		err = p.downloadPlaylist(item.StationID)
		if err != nil {
			p.recover(err)
			fmt.Printf("Error: %s\n", err)
			if err.Error() == "Recieved Stop Signal" {
				os.Exit(0)
			}
		}
	}
}

func (p *panrip) downloadPlaylist(playlistId string) error {
	t := time.NewTimer(15 * time.Minute)
	for {
		playlist, err := p.pandora.StationGetPlaylist(playlistId)
		if err != nil {
			return err
		}
		for _, item := range playlist.Result.Items {
			select {
			case <-p.stop:
				return fmt.Errorf("Recieved Stop Signal")
			case <-t.C:
				return fmt.Errorf("15 minute timeout: no new songs on playlist")
			default:
				if item.SongName == "" {
					continue
				}
				if p.Verbose {
					fmt.Printf("%s - %s", item.ArtistName, item.SongName)
				}
				err = p.download(item.AudioURLMap["highQuality"].AudioURL, item.ArtistName, item.SongName, item.AlbumName, item.AlbumArtURL)
				if err == nil {
					if !t.Stop() {
						<-t.C
					}
					t.Reset(15 * time.Minute)
					if p.Verbose {
						fmt.Println()
					}
				} else {
					if err.Error() == "already downloaded" {
						if p.Verbose {
							fmt.Println(" (already downloaded)")
						}
						time.Sleep(10 * time.Second)
						continue
					}
					if p.Verbose {
						fmt.Println()
					}
					return err
				}
				time.Sleep(30 * time.Second)
			}
		}
	}
}

func (p *panrip) download(url, artist, song, album, albumImg string) error {
	artist = strings.Replace(artist, "/", "-", -1)
	song = strings.Replace(song, "/", "-", -1)
	r := strings.NewReplacer("/", "", "\\", "", "?", "", "%", "", "*", "", ":", "", "|", "", "\"", "", "<", "", ">", "", ".", "")
	base := r.Replace(song)
	if _, err := os.Stat(p.Out + "/" + artist + "/" + base + ".mp3"); err == nil {
		return fmt.Errorf("already downloaded")
	}
	os.MkdirAll(p.Out+"/"+artist, 0744)
	outFile, err := os.Create(p.Out + "/" + artist + "/" + base + ".mp4")
	if err != nil {
		return err
	}
	defer outFile.Close()
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return err
	}
	cmd := exec.Command("ffmpeg", "-i", p.Out+"/"+artist+"/"+base+".mp4", p.Out+"/"+artist+"/"+base+".mp3")
	err = cmd.Run()
	if err != nil {
		return err
	}
	os.Remove(p.Out + "/" + artist + "/" + base + ".mp4")
	tag, err := id3v2.Open(p.Out+"/"+artist+"/"+base+".mp3", id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()
	tag.SetArtist(artist)
	tag.SetTitle(song)
	tag.SetAlbum(album)
	if artwork, err := p.getArtwork(albumImg); err == nil {
		pic := id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    "image/jpeg",
			PictureType: id3v2.PTFrontCover,
			Description: "Front cover",
			Picture:     artwork,
		}
		tag.AddAttachedPicture(pic)
	}
	if err = tag.Save(); err != nil {
		return err
	}
	return nil
}
func (p panrip) getArtwork(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
