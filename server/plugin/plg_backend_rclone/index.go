package plg_backend_rclone

import (
	"bytes"
	"context"
	"errors"
	. "github.com/mickael-kerjean/filestash/server/common"
	_ "github.com/rclone/rclone/backend/all"
	rcloneFs "github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

func init() {
	Backend.Register("rclone", Rclone{})
}

type Rclone struct {
	fs      rcloneFs.Fs
	storage *Storage
	remote  string
}

func (r Rclone) Init(params map[string]string, app *App) (IBackend, error) {
	p := struct {
		config   string
		password string
		storage  string
	}{
		params["config"],
		params["password"],
		params["storage"],
	}

	ctx := context.Background()

	s, err := NewStorage(p.config, p.password)
	if err != nil {
		return nil, err
	}
	config.SetData(s)
	f, err := rcloneFs.NewFs(ctx, p.storage)
	if err != nil {
		return nil, err
	}
	// TODO: how to use connection cache?
	return Rclone{
		fs:      f,
		storage: s,
		remote:  p.storage,
	}, nil
}

func (r Rclone) LoginForm() Form {
	return Form{
		Elmnts: []FormElement{
			{
				Name:  "type",
				Type:  "hidden",
				Value: "rclone",
			},
			{
				Name:        "config",
				Type:        "long_text",
				Placeholder: "Encrypted config for rclone",
				Description: "Encrypted config for rclone",
				Required:    true,
			},
			{
				Name:        "password",
				Type:        "password",
				Placeholder: "Password for the rclone config",
				Required:    true,
			},
			{
				Name:        "storage",
				Type:        "text",
				Placeholder: "Storage from the config",
				Description: "Storage from the config",
			},
		},
	}
}

func (r Rclone) Ls(p string) ([]os.FileInfo, error) {
	ctx := context.Background()
	p = strings.TrimRight(p, "/")
	entities, err := r.fs.List(ctx, p)
	if err != nil {
		return nil, err
	}
	files := make([]os.FileInfo, 0, len(entities))
	for _, entity := range entities {
		switch e := entity.(type) {
		case *rcloneFs.Dir:
			files = append(files, File{
				FName:     path.Base(e.String()),
				FType:     "directory",
				FTime:     e.ModTime(ctx).Unix(),
				FSize:     0,
				FPath:     e.Remote(),
				CanRename: nil,
				CanMove:   nil,
				CanDelete: nil,
			})
		default:
			files = append(files, File{
				FName:     path.Base(entity.String()),
				FType:     "file",
				FTime:     entity.ModTime(ctx).Unix(),
				FSize:     entity.Size(),
				FPath:     entity.Remote(),
				CanRename: nil,
				CanMove:   nil,
				CanDelete: nil,
			})
		}

	}
	return files, nil
}

func (r Rclone) Cat(p string) (io.ReadCloser, error) {
	ctx := context.Background()

	obj, err := r.fs.NewObject(ctx, p)
	if err != nil {
		return nil, err
	}

	return obj.Open(ctx)
}

func (r Rclone) Mkdir(p string) error {
	ctx := context.Background()
	return r.fs.Mkdir(ctx, p)
}

func (r Rclone) Rm(p string) error {
	ctx := context.Background()

	isFile, err := r.isFile(ctx, p)
	if err != nil {
		return err
	}
	if isFile {
		return r.rmFile(ctx, p)
	}

	return r.rmDir(ctx, p)
}

func (r Rclone) isFile(ctx context.Context, p string) (bool, error) {
	p = strings.TrimRight(p, "/")
	base, _ := path.Split(p)
	base = strings.TrimRight(base, "/")
	entities, err := r.fs.List(ctx, base)
	if err != nil {
		return false, err
	}
	for _, entity := range entities {
		if strings.Trim(entity.Remote(), "/") != strings.Trim(p, "/") {
			continue
		}
		switch entity.(type) {
		case *rcloneFs.Dir:
			return false, nil
		default:
			return true, nil
		}
	}
	return false, rcloneFs.ErrorObjectNotFound
}

func (r Rclone) rmDir(ctx context.Context, p string) error {
	p = strings.TrimRight(p, "/")
	return operations.Purge(ctx, r.fs, p)
}

func (r Rclone) rmFile(ctx context.Context, p string) error {
	p = strings.TrimRight(p, "/")
	obj, err := r.fs.NewObject(ctx, p)
	if errors.Is(err, rcloneFs.ErrorObjectNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	return obj.Remove(ctx)
}

func (r Rclone) Mv(from, to string) error {
	ctx := context.Background()

	srcObj, err := r.fs.NewObject(ctx, from)
	if err != nil {
		return err
	}
	dataReader, err := srcObj.Open(ctx)
	if err != nil {
		return err
	}

	destObj := object.NewStaticObjectInfo(to, srcObj.ModTime(ctx), srcObj.Size(), true, nil, r.fs)
	_, err = r.fs.Put(ctx, dataReader, destObj)
	if err != nil {
		return err
	}

	return srcObj.Remove(ctx)
}

func (r Rclone) Save(p string, content io.Reader) error {
	ctx := context.Background()
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	dataReader := bytes.NewReader(data)
	src := object.NewStaticObjectInfo(p, time.Now(), int64(len(data)), true, nil, r.fs)
	_, err = r.fs.Put(ctx, dataReader, src)
	return err
}

func (r Rclone) Touch(p string) error {
	ctx := context.Background()
	var data []byte
	src := object.NewStaticObjectInfo(p, time.Now(), int64(len(data)), true, nil, r.fs)
	_, err := r.fs.Put(ctx, bytes.NewBuffer(data), src)
	return err
}
