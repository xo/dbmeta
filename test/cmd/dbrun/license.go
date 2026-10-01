package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// A product that does not start without a license file, such as Stardog,
// names where it reads the file in container.Server.License. A person
// downloads the file, and dbrun mounts it. dbrun finds it in the first of two
// places that has one, and lists the server only while one does, the way a
// hosted service appears only with its credential. See D117 and D118.
//
//  1. The path in the variable DBMETA_<PRODUCT>_LICENSE.
//  2. The file <product> in $XDG_CONFIG_HOME/dbmeta/licenses.

// licenseEnv names the variable that holds the path of a product's license
// file.
func licenseEnv(product string) string {
	return "DBMETA_" + strings.ToUpper(product) + "_LICENSE"
}

// licenseDir is the directory of license files.
func licenseDir() string {
	return filepath.Join(configDir(), "dbmeta", "licenses")
}

// resolveLicense finds the license file of a product. It returns false when
// no place has one, which is not an error: the server is then absent. An
// error is a place that names a file that cannot be used.
func resolveLicense(product string) (string, bool, error) {
	if p := strings.TrimSpace(os.Getenv(licenseEnv(product))); p != "" {
		if err := usableFile(p); err != nil {
			return "", false, fmt.Errorf("%s names %s: %w", licenseEnv(product), p, err)
		}
		return p, true, nil
	}
	p := filepath.Join(licenseDir(), product)
	switch err := usableFile(p); {
	case errors.Is(err, fs.ErrNotExist):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return p, true, nil
}

// usableFile says why a path cannot be mounted as a license file, or nil
// when it can.
func usableFile(p string) error {
	info, err := os.Stat(p)
	switch {
	case err != nil:
		return fmt.Errorf("reading the license file: %w", err)
	case !info.Mode().IsRegular():
		return fmt.Errorf("%s is not a regular file", p)
	}
	return nil
}

// licenseMount is the flag that mounts a license file where the product
// reads it, and cannot be written from inside.
func licenseMount(host, inside string) []string {
	return []string{"--volume", host + ":" + inside + ":ro"}
}
