# Documentation Site

## Developing And Testing

The [documentation website](https://argo-cd.readthedocs.io/) is built using `mkdocs` and `mkdocs-material`.

To test:

```bash
make serve-docs
```
Once running, you can view your locally built documentation at [http://0.0.0.0:8000/](http://0.0.0.0:8000/).
Making changes to documentation will automatically rebuild and refresh the view.

Before submitting a PR build the website, to verify that there are no errors building the site
```bash
make build-docs
```

If you want to build and test the site directly on your local machine without the use of docker container, follow the below steps:

1. Install the `mkdocs` using the `pip` command
    ```bash
    pip install mkdocs
    ```
2. Install the required dependencies using the below command
   ```bash
   pip install $(mkdocs get-deps)
    ```
3. Build the docs site locally from the root
   ```bash
   make build-docs-local
   ``` 
4. Start the docs site locally
   ```bash
   make serve-docs-local
   ```

## Analytics

!!! tip
    Don't forget to disable your ad-blocker when testing.

We collect [Google Analytics](https://analytics.google.com/analytics/web/#/report-home/a105170809w198079555p192782995).
