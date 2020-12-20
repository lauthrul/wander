package ui

import (
	"encoding/json"
	"fmt"
	"github.com/faiface/beep/speaker"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"time"
	"wander/model"
)

const (
	TextPlay     = "▶"
	TextPause    = "||"
	TextPlayPrev = "◀◀"
	TextPlayNext = "▶▶"
)

type MyMainWindow struct {
	*walk.MainWindow

	// ui
	lbPlayList        *walk.ListBox
	lbTrackList       *walk.ListBox
	lblCurrentPlaying *walk.LinkLabel
	imgCover          *walk.ImageView
	lblName           *walk.Label
	btnPrev           *walk.PushButton
	btnPlay           *walk.PushButton
	btnNext           *walk.PushButton

	// data
	playList  *PlaylistModel
	musicList *TrackModel

	// manager
	pm *model.PlayerManager
	ch chan model.PlayAction
}

func (mw *MyMainWindow) init() {
	go func() {
		for {
			select {
			case status := <-mw.ch:
				fmt.Println(mw.pm.CurrentMusic, status)
				switch status {
				case model.PlayActionStop:
					// DO NOTHING
				case model.PlayActionPlay, model.PlayActionPause:
					text := TextPause
					if status == model.PlayActionPause {
						text = TextPlay
					}
					mw.btnPlay.SetText(text)
					mw.lblCurrentPlaying.SetText(fmt.Sprintf("当前播放： <a>%s</a>", mw.lblName.Text()))
				case model.PlayActionNext:
					mw.onPlayNext()
				}
			case <-time.After(time.Second):
				if mw.pm.CurrentMusic != nil {
					if mw.pm.CurrentMusic.Streamer != nil {
						speaker.Lock()
						if !mw.pm.CurrentMusic.Ctrl.Paused {
							fmt.Println(mw.pm.CurrentMusic, mw.pm.CurrentMusic.Format.SampleRate.D(mw.pm.CurrentMusic.Streamer.Position()).Round(time.Second))
						}
						speaker.Unlock()
					}
				}
			}
		}
	}()
}

func (mw *MyMainWindow) updateControlPanel(music *model.MusicInfo) {
	img, err := walk.NewImageFromFile(music.MusicPicLocal)
	if err != nil {
		fmt.Println("load music pic err:", err)
		return
	}
	mw.imgCover.SetImage(img)
	mw.lblName.SetText(music.Name + " - " + music.ArtistsName)

	if mw.pm.CurrentMusic != nil {
		if mw.pm.CurrentMusic.MusicLocal == music.MusicLocal {
			mw.btnPlay.SetText(TextPause)
		} else {
			mw.btnPlay.SetText(TextPlay)
		}
	}
}

func (mw *MyMainWindow) onGotoTackList(link *walk.LinkLabelLink) {
	if mw.pm.CurrentMusic == nil {
		return
	}

	idx := -1
	for i, m := range mw.musicList.items {
		if m.ID == mw.pm.CurrentMusic.ID {
			idx = i
			break
		}
	}
	mw.lbTrackList.SetCurrentIndex(idx)
}

func (mw *MyMainWindow) onPlaylistChanged() {
	mw.Synchronize(func() {
		idx := mw.lbPlayList.CurrentIndex()
		if idx < 0 || idx >= len(mw.playList.items) {
			return
		}
		item := mw.playList.items[idx]
		url := fmt.Sprintf(model.Playlist, item.ID)
		data, _, err := model.HttpDoTimeout(nil, "GET", url, nil, 30*time.Second)
		if err != nil {
			return
		}
		var playlist model.PlaylistResp
		err = json.Unmarshal(data, &playlist)
		if err != nil {
			return
		}
		if playlist.Code != 200 {
			return
		}

		mw.musicList.items = model.WalkPlaylist(&playlist)
		mw.musicList.PublishItemsReset()
	})
}

func (mw *MyMainWindow) onTrackListChanged() {
	mw.Synchronize(func() {
		var err error
		idx := mw.lbTrackList.CurrentIndex()
		if idx < 0 || idx >= len(mw.musicList.items) {
			return
		}
		music := mw.musicList.items[idx]
		fileName := fmt.Sprintf("%s-%s", music.Name, music.ArtistsName)
		res, ok := model.CheckCaches("cache", fileName, model.CachePic)
		if ok {
			music.MusicPicLocal = res[model.CachePic]
		} else {
			// download music pic
			music.MusicPicLocal, err = model.Download(music.MusicPic, "/", fileName)
			if err != nil {
				return
			}
		}
		mw.updateControlPanel(music)
	})
}

func (mw *MyMainWindow) play(idx int) {
	if idx < 0 || idx > len(mw.musicList.items) {
		fmt.Println("playlist idx err:", idx)
		return
	}
	music := mw.musicList.items[idx]
	if music.MusicLocal == "" {
		fileName := fmt.Sprintf("%s-%s", music.Name, music.ArtistsName)
		res, ok := model.CheckCaches("cache", fileName, model.CacheMusic)
		if ok {
			music.MusicLocal = res[model.CacheMusic]
		} else {
			// download music
			link := fmt.Sprintf(model.LinkUrl, music.ID)
			data, _, err := model.HttpDoTimeout(nil, "GET", link, nil, 2*time.Minute)
			if err != nil {
				return
			}
			var linkInfo model.LinkInfo
			err = json.Unmarshal(data, &linkInfo)
			if err != nil {
				return
			}
			if linkInfo.Code != 200 {
				fmt.Println(err)
				return
			}
			music.MusicUrl = linkInfo.Data.Url
			music.MusicLocal, err = model.Download(music.MusicUrl, "/", fileName)
			if err != nil {
				return
			}
		}
	}
	// play music
	mw.pm.Play(music)
}

func (mw *MyMainWindow) onPlayPrev() {
	mw.Synchronize(func() {
		idx := mw.lbTrackList.CurrentIndex() - 1
		if idx < 0 {
			idx = len(mw.musicList.items) - 1
		}
		mw.lbTrackList.SetCurrentIndex(idx)
		mw.play(idx)
	})
}

func (mw *MyMainWindow) onPlay() {
	mw.Synchronize(func() {
		mw.play(mw.lbTrackList.CurrentIndex())
	})
}

func (mw *MyMainWindow) onPlayNext() {
	mw.Synchronize(func() {
		idx := mw.lbTrackList.CurrentIndex() + 1
		max := len(mw.musicList.items) - 1
		if idx > max {
			idx = 0
		}
		mw.lbTrackList.SetCurrentIndex(idx)
		mw.play(idx)
	})
}

func Run() {

	walk.Resources.SetRootDirPath("cache")

	mw := &MyMainWindow{
		playList: NewPlaylist(),
		ch:       make(chan model.PlayAction),
	}
	mw.musicList = NewTrackList(mw)
	mw.pm = model.NewPlayerManager(mw.ch)

	mw.init()

	MainWindow{
		AssignTo: &mw.MainWindow,
		Title:    "wander",
		MinSize:  Size{Width: 500, Height: 300},
		MaxSize:  Size{Width: 500, Height: 300},
		Size:     Size{Width: 500, Height: 300},
		Layout:   HBox{},
		Children: []Widget{
			HSplitter{
				//MinSize: Size{Width: 300},
				//MaxSize: Size{Width: 300},
				Children: []Widget{
					// 播放列表
					ListBox{
						AssignTo: &mw.lbPlayList,
						MinSize:  Size{Width: 100},
						MaxSize:  Size{Width: 100},
						Model:    mw.playList,
						//CurrentIndex:          0,
						OnCurrentIndexChanged: mw.onPlaylistChanged,
					},
					// 歌单
					ListBox{
						AssignTo: &mw.lbTrackList,
						MinSize:  Size{Width: 200, Height: 32},
						//MaxSize:  Size{Width: 200, Height: 32},
						Model:                 mw.musicList,
						OnCurrentIndexChanged: mw.onTrackListChanged,
					},
				},
			},
			Composite{
				Layout: VBox{},
				Children: []Widget{
					LinkLabel{
						AssignTo:        &mw.lblCurrentPlaying,
						Text:            "当前播放：",
						OnLinkActivated: mw.onGotoTackList,
					},
					ImageView{
						AssignTo: &mw.imgCover,
						//Background: SolidColorBrush{Color: walk.RGB(0, 0, 0)},
						Image: "img.jpg",
						//MaxSize: Size{Width: 200, Height: 200},
						MinSize: Size{Width: 200, Height: 200},
						//Margin:  10,
						Mode: ImageViewModeZoom,
					},
					Label{
						AssignTo:  &mw.lblName,
						Alignment: AlignHCenterVCenter,
						//Font:      Font{Family: "微软雅黑", Bold: true},
						Text: "音乐的力量",
					},
					Composite{
						MaxSize: Size{0, 32},
						Layout:  Grid{Columns: 4},
						Children: []Widget{
							PushButton{
								AssignTo:  &mw.btnPrev,
								Text:      TextPlayPrev,
								OnClicked: mw.onPlayPrev,
							},
							PushButton{
								AssignTo:  &mw.btnPlay,
								Text:      TextPlay,
								OnClicked: mw.onPlay,
							},
							PushButton{
								AssignTo:  &mw.btnNext,
								Text:      TextPlayNext,
								OnClicked: mw.onPlayNext,
							},
						},
					},
				},
			},
		},
	}.Run()
}
