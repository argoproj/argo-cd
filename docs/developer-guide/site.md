# Site

## Developing And Testing

The website is built using `mkdocs` and `mkdocs-material`.

To test:

```bash
make serve-docs
```
Once running, you can view your locally built documentation at [http://0.0.0.0:8000/](http://0.0.0.0:8000/).
Make a change to documentation and the website will rebuild and refresh the view.

Before submitting a PR build the website, to verify that there are no errors building the site
```bash
make build-docs
```

## Analytics

!!! tip
    Don't forget to disable your ad-blocker when testing.

We collect [Google Analytics](https://analytics.google.com/analytics/web/#/report-home/a105170809w198079555p192782995).
