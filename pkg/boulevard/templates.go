package boulevard

import (
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/fs"
	"strings"
	texttemplate "text/template"
)

var functions = map[string]any{
	"join": strings.Join,
}

func LoadTextTemplates(filesystem fs.FS, patterns ...string) (*texttemplate.Template, error) {
	rootTpl := texttemplate.New("")

	rootTpl = rootTpl.Option("missingkey=error")
	rootTpl = rootTpl.Funcs(functions)

	// We do not use tpl.ParseFS becauses it forces the use of glob patterns and
	// for some reason name all templates as their basename instead of their
	// relative path in the filesystem.
	//
	// Also doing it ourselves allows us to drop file extensions.

	err := walkFS(filesystem, ".txt.gotpl",
		func(filename string, data []byte) error {
			tpl := rootTpl.New(filename)
			if _, err := tpl.Parse(string(data)); err != nil {
				return fmt.Errorf("cannot parse template %q: %w", filename, err)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	return rootTpl, nil
}

func LoadHTMLTemplates(filesystem fs.FS, patterns ...string) (*htmltemplate.Template, error) {
	rootTpl := htmltemplate.New("")

	rootTpl = rootTpl.Option("missingkey=error")
	rootTpl = rootTpl.Funcs(functions)

	err := walkFS(filesystem, ".html.gotpl",
		func(filename string, data []byte) error {
			tpl := rootTpl.New(filename)
			if _, err := tpl.Parse(string(data)); err != nil {
				return fmt.Errorf("cannot parse template %q: %w", filename, err)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	return rootTpl, nil
}

func walkFS(filesystem fs.FS, suffix string, fn func(string, []byte) error) error {
	// fs, embed, template, there is no shortage of badly designed modules in
	// the standard library. embed.FS gives you a fs.FS object instead of a
	// simple map associating filename and file content. Why make it easy right?

	return fs.WalkDir(filesystem, ".",
		func(filePath string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.IsDir() {
				return nil
			}

			if !strings.HasSuffix(filePath, suffix) {
				return nil
			}

			end := len(filePath) - len(suffix)
			filename := filePath[:end]

			var data []byte

			file, err := filesystem.Open(filePath)
			if err != nil {
				return fmt.Errorf("cannot open %q: %w", filePath, err)
			}

			data, err = io.ReadAll(file)
			if err != nil {
				file.Close()
				return fmt.Errorf("cannot read %q: %w", filePath, err)
			}

			file.Close()

			return fn(filename, data)
		})
}
