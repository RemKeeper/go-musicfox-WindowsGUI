package entry

import (
	_ "embed"
	"encoding/json"
	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/internal/ui"
	"github.com/go-musicfox/go-musicfox/utils"
	"github.com/go-musicfox/netease-music/service"
	"log"
	"net/http"
	"time"
)

//go:embed html/playlist.html
var playListHtml []byte

func WebHookServer() {
	http.HandleFunc("/search", SearchMusicHandler)
	http.HandleFunc("/next", NextMusicHandler)
	http.HandleFunc("/playList", PlayListHandler)
	http.HandleFunc("/music", WebServer)
	http.HandleFunc("/lyrics", LyricHandler)

	http.HandleFunc("/getPlayIndex", GetPlayIndex)

	http.ListenAndServe(":99", nil)
}

type LyricDate struct {
	PlayedTime time.Duration
	Duration   time.Duration
	Lyric      [5]string
}

func LyricHandler(writer http.ResponseWriter, _ *http.Request) {
	lyricPayload := LyricDate{
		PlayedTime: ui.GlobalPlayer.PlayedTime(),
		Duration:   ui.GlobalPlayer.CurMusic().Duration,
		Lyric:      ui.GlobalPlayer.Lyrics(),
	}
	marshal, err := json.Marshal(lyricPayload)
	if err != nil {
		return
	}
	writer.Write(marshal)

}

type PlayIndex struct {
	Index  int
	Length int
}

func GetPlayIndex(writer http.ResponseWriter, _ *http.Request) {
	marshal, err := json.Marshal(PlayIndex{
		Index:  ui.GlobalPlayer.CurSongIndex(),
		Length: len(ui.GlobalPlayer.Playlist()),
	})
	if err != nil {
		return
	}
	writer.Write(marshal)
}

func WebServer(writer http.ResponseWriter, _ *http.Request) {
	writer.Write(playListHtml)
}

type DisplayPlayList struct {
	PlayList       []structs.Song
	CurSongIndex   int
	PlayListLength int
}

func PlayListHandler(writer http.ResponseWriter, _ *http.Request) {
	marshal, err := json.Marshal(DisplayPlayList{
		PlayList:       ui.GlobalPlayer.Playlist(),
		CurSongIndex:   ui.GlobalPlayer.CurSongIndex(),
		PlayListLength: len(ui.GlobalPlayer.Playlist()),
	})
	if err != nil {
		return
	}
	writer.Write(marshal)
}

func NextMusicHandler(_ http.ResponseWriter, _ *http.Request) {
	ui.GlobalPlayer.NextSong(true)
}

var ChooseSongIndex = -1

func SearchMusicHandler(_ http.ResponseWriter, r *http.Request) {
	if ChooseSongIndex < 0 || ChooseSongIndex < ui.GlobalPlayer.CurSongIndex() {
		ChooseSongIndex = ui.GlobalPlayer.CurSongIndex()
	}
	keyword := r.URL.Query().Get("keyword")
	searchService := service.SearchService{
		S: keyword,
	}
	var (
		code     float64
		response []byte
	)
	code, response = searchService.Search()

	codeType := utils.CheckCode(code)
	switch codeType {
	case utils.UnknownError:
		log.Println("未知错误，请稍后再试~")
	case utils.NetworkError:
		log.Println("网络异常，请稍后再试~")
	case utils.Success:
		song := utils.GetSongsOfSearchResult(response)[0]
		ChooseSongIndex++
		ui.GlobalPlayer.AddSong([]structs.Song{song}, ChooseSongIndex)
	}
}
