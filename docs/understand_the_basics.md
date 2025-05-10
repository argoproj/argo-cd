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

To get started with contributing, it's helpful to familiarize yourself with the project structure. Some key folders/files to check:
- **`cmd/`**: Contains the main entry points for the various Argo CD commands/tools.
- **`pkg/`**: Contains reusable packages used throughout the project.
- **`manifests/`**: Kubernetes manifests for deploying Argo CD.
- **`docs/`**: Documentation files explaining various components.
- **`README.md`**: Overview of the project, including setup and usage instructions.

---

## Run the Project Locally

To run Argo CD locally, please refer to the official developer guide:
[Running Locally](https://argo-cd.readthedocs.io/en/stable/developer-guide/running-locally/)

This guide provides comprehensive instructions for setting up and running Argo CD in a local environment. It ensures you follow the recommended practices and avoid common pitfalls.

---

## Start Contributing

Once you're comfortable navigating the codebase, look for issues tagged as **"good first issue"** in the [Argo CD GitHub Issues](https://github.com/argoproj/argo-cd/issues). These are beginner-friendly tasks to help you get started.

Good first issues may be hard to find in a large project like Argo CD with many contributors. If you're unable to find one, follow these steps:

1. **Pick a Component**:
   - Choose an Argo CD component to focus on (e.g., `repo-server`, `controller`, etc.).

2. **Explore the Codebase**:
   - Dive into the code for that component.
   - Look for opportunities to refactor or add tests.

3. **Sort Issues by Label**:
   - Use the component’s label to filter issues in the repository.
   - Pick a relevant issue that matches your interests and skills.

By following these steps, you'll gradually familiarize yourself with the project and its contribution process, even if you don't start with a designated "good first issue."

---

## Additional Learning Resources

Here are some additional resources to help you get started with contributing to Argo CD:

- **Argo CD Developer Guide**: [https://argo-cd.readthedocs.io/en/stable/developer-guide/](https://argo-cd.readthedocs.io/en/stable/developer-guide/)
- **Kubernetes Basics**: If you're new to Kubernetes, explore the [interactive tutorials](https://kubernetes.io/docs/tutorials/) on the official website.
- **GitOps Principles**: Deep dive into GitOps principles with resources like [GitOps.tech](https://www.gitops.tech/).

---

## Contribution Etiquette

When contributing to Argo CD, please keep the following in mind:

1. **Follow the Project's Contribution Guidelines**:
   - Familiarize yourself with the [Contributing Guide](https://github.com/argoproj/argo-cd/blob/master/CONTRIBUTING.md).

2. **Communicate Respectfully**:
   - Open-source projects thrive on collaboration. Be respectful and constructive in all communication.

3. **Ask for Help**:
   - If you're unsure about something, don't hesitate to ask questions in the project's [Slack community](https://argoproj.github.io/community/join-slack/) or GitHub Discussions.

4. **Test Your Changes**:
   - Ensure that any code or documentation changes are tested and verified before submitting a pull request.

---