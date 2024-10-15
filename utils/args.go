package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mandelsoft/filepath/pkg/filepath"
	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"ocm.software/ocm/api/utils/runtime"
)

func Args(p PathCompleter, raw json.RawMessage) ([]string, error) {
	single, err := Arg(p, raw)
	if err == nil {
		return []string{single}, nil
	}

	var list []json.RawMessage
	err = runtime.DefaultYAMLEncoding.Unmarshal(raw, &list)
	if err != nil {
		return nil, fmt.Errorf("simple arg or list of args required")
	}

	var args []string
	for i, a := range list {
		single, err = Arg(p, a)
		if err != nil {
			return nil, errors.Wrapf(err, "argument %d", i)
		}
		args = append(args, single)
	}
	return args, nil
}

type ComplexArg struct {
	Path          string `json:"path,omitempty"`
	GoPackagePath string `json:"gopkgpath,omitempty"`
}

func Arg(p PathCompleter, a json.RawMessage) (string, error) {
	var simple string
	err := runtime.DefaultYAMLEncoding.Unmarshal(a, &simple)
	if err == nil {
		return simple, nil
	} else {
		var arg ComplexArg
		err := runtime.DefaultYAMLEncoding.Unmarshal(a, &arg)
		if err != nil {
			return "", err
		}
		if arg.GoPackagePath == "" && arg.Path == "" {
			return "", errors.Newf("path or gopkgpath must be set")
		}
		if arg.GoPackagePath != "" && arg.Path != "" {
			return "", errors.Newf("either path or gopkgpath must be set")
		}
		if arg.Path != "" {
			return p.Path(arg.Path), nil
		} else {
			path := p.Path(arg.GoPackagePath)
			if !filepath.IsAbs(path) {
				_, c := vfs.Components(osfs.OsFs, path)
				if c[0] != "." {
					path = strings.Join(append([]string{"."}, c...), string(os.PathSeparator))
				}
			}
			return path, nil
		}
	}
}
