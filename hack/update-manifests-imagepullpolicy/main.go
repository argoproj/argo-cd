package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"strings"
)

const AutoGenMsg = "# This is an auto-generated file. DO NOT EDIT"

func main() {
	manifestFilePathPtr := flag.String("manifest", "", "")
	imagePtr := flag.String("image", "quay.io/argoproj/argocd:latest", "")
	imagePullPlolicyPtr := flag.String("image-pull-policy", "Always", "")

	flag.Parse()

	if manifestFilePathPtr == nil || *manifestFilePathPtr == "" {
		fmt.Println("no manifest file path specified")
		os.Exit(1)
	}

	manifestFileContent, err := os.ReadFile(*manifestFilePathPtr)
	if err != nil {
		fmt.Printf("failed to read manifest file %s: %v\n", *manifestFilePathPtr, err)
		os.Exit(1)
	}

	manifests := strings.Split(string(manifestFileContent), "---")
	convertedManifests := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		manifest = strings.TrimSpace(manifest)

		unmarshalledManifest := yaml.MapSlice{}
		if err := yaml.Unmarshal([]byte(manifest), &unmarshalledManifest); err != nil {
			fmt.Println("failed to unmarshal manifest file:", err)
			os.Exit(1)
		}

		loop(unmarshalledManifest, *imagePtr, *imagePullPlolicyPtr)

		marshalledManifest, err := yaml.Marshal(unmarshalledManifest)
		if err != nil {
			fmt.Printf("failed to marshal manifest file: %v\n", err)
			os.Exit(1)
		}

		convertedManifests = append(convertedManifests, strings.TrimSpace(string(marshalledManifest)))
	}

	convertedMergedManifest := strings.Join(convertedManifests, "\n---\n")
	convertedMergedManifest = fmt.Sprintf("%s\n%s\n", AutoGenMsg, convertedMergedManifest)

	manifestFileContent = []byte(convertedMergedManifest)

	if err = os.WriteFile(*manifestFilePathPtr, manifestFileContent, 0644); err != nil {
		fmt.Printf("failed to write manifest file %s: %v\n", *manifestFilePathPtr, err)
		os.Exit(1)
	}
}

func loop(obj yaml.MapSlice, imageName, imagePullPolicy string) {
	isTarget := false
	for _, kv := range obj {
		k, ok := kv.Key.(string)
		if !ok || k != "image" {
			continue
		}

		v, ok := kv.Value.(string)
		if !ok || v != imageName {
			continue
		}

		isTarget = true
	}

	if isTarget {
		for i, kv := range obj {
			k, ok := kv.Key.(string)
			if ok && k == "imagePullPolicy" {
				kv.Value = imagePullPolicy
			}
			obj[i] = kv
		}
	}

	for _, kv := range obj {
		obj, ok := kv.Value.(yaml.MapSlice)
		if ok {
			loop(obj, imageName, imagePullPolicy)
			continue
		}

		items, ok := kv.Value.([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			obj, ok := item.(yaml.MapSlice)
			if ok {
				loop(obj, imageName, imagePullPolicy)
			}
		}
	}
}
