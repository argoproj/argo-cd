# Understand The Basics

Before effectively using Argo CD, it is necessary to understand the underlying technology that the platform is built on. It is also necessary to understand the features being provided to you and how to use them. The section below provides some useful links to build up this understanding.
 
## Learn The Fundamentals

* Go through the online Docker and Kubernetes tutorials:
	* [A Beginner-Friendly Introduction to Containers, VMs and Docker](https://medium.freecodecamp.org/a-beginner-friendly-introduction-to-containers-vms-and-docker-79a9e3e119b)
	* [Introduction to Kubernetes](https://www.edx.org/course/introduction-to-kubernetes)
	* [Tutorials](https://kubernetes.io/docs/tutorials/)
* Depending on how you plan to template your applications:
    * [Kustomize](https://kustomize.io) 
    * [Helm](https://helm.sh)
* If you're integrating with a CI tool:
	* [GitHub Actions Documentation](https://docs.github.com/en/actions)
	* [Jenkins User Guide](https://www.jenkins.io/doc/book/)

# Argo CD Contribution Guide

## Explore the Project Structure
In VS Code, take a look at the structure of the project. Some key folders/files to check:

- **`cmd/`**: Contains the main entry points for the various Argo CD commands/tools.
- **`pkg/`**: Contains reusable packages used throughout the project.
- **`manifests/`**: Kubernetes manifests for deploying Argo CD.
- **`docs/`**: Documentation files explaining various components.
- **`README.md`**: Overview of the project, including setup and usage instructions.

---

## Run the Project Locally
To understand how the project works, you can run it locally:

### Install Dependencies
Argo CD is written in Go, so you'll need Go installed (version 1.19 or later). Install it if you haven't already:
```bash
sudo apt update
sudo apt install golang
```

### Run the Project
Run the following command to start the Argo CD CLI tool:
```bash
go run ./cmd/argocd
```
This will start the Argo CD CLI tool. You'll need Kubernetes access to test its functionality.

---

## Read Key Documentation
Look for documentation within the **`docs/`** folder or online in the [Argo CD GitHub Wiki](https://github.com/argoproj/argo-cd/wiki). This will help you understand the architecture and functionality of the project.

---

## Debug or Add Logs (Optional)
To see how specific parts of the code work, you can add `fmt.Println()` statements in the relevant parts of the codebase and re-run the project.

---

## Search for Specific Functionality
Use VS Code's global search feature (`Ctrl+Shift+F` or `Cmd+Shift+F`) to find specific functions, keywords, or concepts (e.g., **sync**, **manifest**, **controller**).

---

## Start Contributing
Once you're comfortable navigating the codebase, look for issues tagged as **"good first issue"** in the [Argo CD GitHub Issues](https://github.com/argoproj/argo-cd/issues). These are beginner-friendly tasks to help you get started.