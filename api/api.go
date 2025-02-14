package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/gorilla/mux"
)

func New() http.Handler {
	router := mux.NewRouter()
	router.Handle("/package/{package}/{version}", http.HandlerFunc(packageHandler))
	return router
}

type npmPackageMetaResponse struct {
	Versions map[string]npmPackageResponse `json:"versions"`
}

type npmPackageResponse struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

type NpmPackageVersion struct {
	Name         string                        `json:"name"`
	Version      string                        `json:"version"`
	Dependencies map[string]*NpmPackageVersion `json:"dependencies"`
}

type packageCacheKey struct {
	name    string
	version string
}

var packageNameToVersionToDeps = make(map[packageCacheKey]*NpmPackageVersion)

func packageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pkgName := vars["package"]
	pkgVersion := vars["version"]

	rootPkg, err := resolveDependencies(pkgName, pkgVersion)
	if err != nil {
		println(err.Error())
		w.WriteHeader(500)
		return
	}

	fmt.Printf("called fetchPackage %v times\n", fetchPackageCount)
	fmt.Printf("called fetchPackageMeta %v times\n", fetchPackageMetaCount)

	stringified, err := json.MarshalIndent(rootPkg, "", "  ")
	if err != nil {
		println(err.Error())
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)

	// Ignoring ResponseWriter errors
	_, _ = w.Write(stringified)
}

func resolveDependencies(packageName string, versionConstraint string) (*NpmPackageVersion, error) {
	pkgMeta, err := fetchPackageMeta(packageName)
	if err != nil {
		return nil, err
	}
	concreteVersion, err := highestCompatibleVersion(versionConstraint, pkgMeta)
	if err != nil {
		return nil, err
	}

	npmPkg, err := fetchPackage(packageName, concreteVersion)
	if err != nil {
		return nil, err
	}
	packageDependencies := make(map[string]*NpmPackageVersion)
	for dependencyName, dependencyVersionConstraint := range npmPkg.Dependencies {
		cacheKey := packageCacheKey{
			name:    dependencyName,
			version: dependencyVersionConstraint,
		}
		cachedDeps := packageNameToVersionToDeps[cacheKey]
		if cachedDeps != nil {
			packageDependencies[dependencyName] = cachedDeps
			continue
		}
		subDeps, err := resolveDependencies(dependencyName, dependencyVersionConstraint)
		if err != nil {
			return nil, err
		}
		packageNameToVersionToDeps[cacheKey] = subDeps
		packageDependencies[dependencyName] = subDeps
	}
	return &NpmPackageVersion{
		Name:         packageName,
		Version:      concreteVersion,
		Dependencies: packageDependencies,
	}, nil
}

func highestCompatibleVersion(constraintStr string, versions *npmPackageMetaResponse) (string, error) {
	constraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		return "", err
	}
	filtered := filterCompatibleVersions(constraint, versions)
	sort.Sort(filtered)
	if len(filtered) == 0 {
		return "", errors.New("no compatible versions found")
	}
	return filtered[len(filtered)-1].String(), nil
}

func filterCompatibleVersions(constraint *semver.Constraints, pkgMeta *npmPackageMetaResponse) semver.Collection {
	var compatible semver.Collection
	for version := range pkgMeta.Versions {
		semVer, err := semver.NewVersion(version)
		if err != nil {
			continue
		}
		if constraint.Check(semVer) {
			compatible = append(compatible, semVer)
		}
	}
	return compatible
}

var fetchPackageCount = 0

func fetchPackage(name, version string) (*npmPackageResponse, error) {
	fetchPackageCount++
	resp, err := http.Get(fmt.Sprintf("https://registry.npmjs.org/%s/%s", name, version))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed npmPackageResponse
	_ = json.Unmarshal(body, &parsed)
	return &parsed, nil
}

var fetchPackageMetaCount = 0

func fetchPackageMeta(p string) (*npmPackageMetaResponse, error) {
	fetchPackageMetaCount++
	resp, err := http.Get(fmt.Sprintf("https://registry.npmjs.org/%s", p))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed npmPackageMetaResponse
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, err
	}

	return &parsed, nil
}
