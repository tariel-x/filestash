package plg_backend_rclone

import (
	"bytes"
	"context"
	"errors"
	. "github.com/mickael-kerjean/filestash/server/common"
	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/cmd"
	rcloneFs "github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"
)

func init() {
	Backend.Register("rclone", Rclone{})
}

type Rclone struct {
	fs     rcloneFs.Fs
	conf   *Storage
	remote string
}

func (r Rclone) Init(params map[string]string, app *App) (IBackend, error) {
	p := struct {
		config   string
		password string
		remote   string
	}{
		params["config"],
		params["password"],
		params["storage"],
	}

	ctx := context.Background()

	conf, err := NewStorage(p.config, p.password)
	if err != nil {
		return nil, err
	}
	config.SetData(conf)
	f, err := rcloneFs.NewFs(ctx, p.remote)
	if err != nil {
		return nil, err
	}
	// TODO: how to use connection cache?
	return Rclone{
		fs:     f,
		conf:   conf,
		remote: p.remote,
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

	src := path.Join(strings.TrimRight(r.remote, "/"), strings.TrimLeft(from, "/")) + "/"
	dest := path.Join(strings.TrimRight(r.remote, "/"), strings.TrimLeft(to, "/")) + "/"

	fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst([]string{src, dest})
	if srcFileName == "" {
		return sync.MoveDir(ctx, fdst, fsrc, true, true)
	}
	return operations.MoveFile(ctx, fdst, fsrc, srcFileName, srcFileName)
}

func (r Rclone) Save(p string, content io.Reader) error {
	ctx := context.Background()

	const (
		kb        = 1 << 10
		mb        = kb << 10
		maxMemory = mb * 50
	)

	data := make([]byte, maxMemory)
	_, err := io.ReadAtLeast(content, data, maxMemory)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	dataReader := io.Reader(bytes.NewReader(data))
	size := int64(len(data))

	if err == nil {
		written, f, saveErr := r.saveBuffer(content, data)
		if saveErr != nil {
			return saveErr
		}
		dataReader = f
		size = int64(written)

		defer func() {
			if err := f.Close(); err != nil {
				slog.Error("error closing temp file", "err", err)
			}
			if err := os.Remove(f.Name()); err != nil {
				slog.Error("error removing temp file", "err", err)
			}
		}()
	}

	src := object.NewStaticObjectInfo(p, time.Now(), size, true, nil, r.fs)
	obj, err := r.fs.Put(ctx, dataReader, src)
	if err != nil {
		return err
	}
	if size != obj.Size() {
		return errors.New("size != obj size")
	}
	return err
}

func (r Rclone) saveBuffer(content io.Reader, data []byte) (int, *os.File, error) {
	f, err := os.CreateTemp("", "filestash-rclone")
	if err != nil {
		return 0, nil, err
	}

	written, err := f.Write(data)
	if err != nil {
		return 0, nil, err
	}
	if written != len(data) {
		return 0, nil, errors.New("written != data")
	}

	copied, err := io.Copy(f, content)
	if err != nil {
		return 0, nil, err
	}

	_, err = f.Seek(0, 0)

	return len(data) + int(copied), f, err
}

func (r Rclone) Touch(p string) error {
	ctx := context.Background()
	var data []byte
	src := object.NewStaticObjectInfo(p, time.Now(), int64(len(data)), true, nil, r.fs)
	_, err := r.fs.Put(ctx, bytes.NewBuffer(data), src)
	return err
}
