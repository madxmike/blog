package hotreload

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

type Service struct {
	templateFSRoot string
	templateFS     fs.FS
	template       *template.Template

	wsConn *websocket.Conn
	wsMu   sync.Mutex
}

func NewService(templateFSRoot string, templateFS fs.FS, template *template.Template) (*Service, error) {
	s := &Service{
		templateFSRoot: templateFSRoot,
		templateFS:     templateFS,
		template:       template,
	}

	err := s.watch()
	if err != nil {
		return nil, fmt.Errorf("could not create new hot reload service: %w", err)
	}

	return s, nil
}

func (s *Service) watch() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("could not begin watching fs: %w", err)
	}

	err = watcher.Add(s.templateFSRoot)
	if err != nil {
		return fmt.Errorf("could not begin watching fs: %w", err)
	}

	err = fs.WalkDir(s.templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		// Skip over the root as we have already added it above
		if path == "." {
			return nil
		}

		if d.IsDir() {
			fsPath := s.templateFSRoot + string(os.PathSeparator) + path
			err := watcher.Add(fsPath)

			fmt.Println(fsPath)
			if err != nil {
				return fmt.Errorf("could not add watcher for `%s`: %w", path, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not begin watching fs: %w", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					panic("fs watcher events failed")
				}

				base := filepath.Base(event.Name)
				fsPath := strings.ReplaceAll(event.Name, s.templateFSRoot+string(os.PathSeparator), "")
				fstat, err := fs.Stat(s.templateFS, fsPath)
				if err != nil {
					log.Println(fmt.Errorf("could not get information for %s in fs: %w", base, err))
					continue
				}

				if fstat.IsDir() && event.Has(fsnotify.Create) {
					watcher.Add(base)
					continue
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Create) {
					s.hotReload(filepath.Base(base))
				}
			case err := <-watcher.Errors:
				fmt.Println(err)
			}
		}
	}()

	return nil
}

func (s *Service) hotReload(fileName string) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	if s.wsConn == nil {
		log.Println(fmt.Errorf("failed to perform hot reload of templates: websocket is nil"))
		return
	}

	reloadedTemplates, err := template.ParseFS(s.templateFS, "*.html", "**/*.html")
	if err != nil {
		log.Println(fmt.Errorf("failed to perform hot reload of templates: %w", err))
		return
	}
	*s.template = *reloadedTemplates

	w, err := s.wsConn.NextWriter(websocket.TextMessage)
	if err != nil {
		panic(fmt.Errorf("failed to perform hot reload of templates: %w", err))
	}

	err = s.template.ExecuteTemplate(w, fileName, nil)
	if err != nil {
		log.Println(fmt.Errorf("failed to perform hot reload of templates: %w", err))
		return
	}
}

func (s *Service) SetWebsocketConn(wsConn *websocket.Conn) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	s.wsConn = wsConn
}
