# Site

## Developing And Testing

The web site is build using `mkdocs` and `mkdocs-material`. 

To test:

```bash
mkdocs serve
```

Check for broken external links:

```bash
find docs -name '*.md' -exec grep -l http {} + | xargs awesome_bot -t 3 --allow-dupe --allow-redirect -w argocd.example.com:443,argocd.example.com,kubernetes.default.svc:443,kubernetes.default.svc,mycluster.com,https://github.com/argoproj/my-private-repository,192.168.0.20,storage.googleapis.com,localhost:8080,localhost:6443,your-kubernetes-cluster-addr,10.97.164.88 --skip-save-results --
```

## Deploying

```bash
mkdocs gh-deploy
```

## Analytics

!!! tip
    Don't forget to disable your ad-blocker when testing.

We collect [Google Analytics](https://analytics.google.com/analytics/web/#/report-home/a105170809w198079555p192782995).