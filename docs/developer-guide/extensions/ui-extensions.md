# UI Extensions

Argo CD web user interface can be extended with additional UI elements. Extensions should be delivered as a javascript file
in the `argocd-server` Pods that are placed in the `/tmp/extensions` directory and starts with `extension` prefix ( matches to `^extension(.*)\.js$` regex ).

```
/tmp/extensions
├── example1
│   └── extension-1.js
└── example2
    └── extension-2.js
```

Extensions are loaded during initial page rendering and should register themselves using API exposed in the `extensionsAPI` global variable. (See
corresponding extension type details for additional information).

The extension should provide a React component that is responsible for rendering the UI element. Extension should not bundle the React library.
Instead extension should use the `react` global variable. You can leverage `externals` setting if you are using webpack:

```js
externals: {
  react: "React";
}
```

## Resource Tab Extensions

Resource Tab extensions is an extension that provides an additional tab for the resource sliding panel at the Argo CD Application details page.

The resource tab extension should be registered using the `extensionsAPI.registerResourceExtension` method:

```typescript
registerResourceExtension(component: ExtensionComponent, group: string, kind: string, tabTitle: string)
```

- `component: ExtensionComponent` is a React component that receives the following properties:

  - application: Application - Argo CD Application resource;
  - resource: State - the kubernetes resource object;
  - tree: ApplicationTree - includes list of all resources that comprise the application;

  See properties interfaces in [models.ts](https://github.com/argoproj/argo-cd/blob/master/ui/src/app/shared/models.ts)

- `group: string` - the glob expression that matches the group of the resource; note: use globstar (`**`) to match all groups including empty string;
- `kind: string` - the glob expression that matches the kind of the resource;
- `tabTitle: string` - the extension tab title.
- `opts: Object` - additional options:
  - `icon: string` - the class name the represents the icon from the [https://fontawesome.com/](https://fontawesome.com/) library (e.g. 'fa-calendar-alt');

Below is an example of a resource tab extension:

```javascript
((window) => {
  const component = () => {
    return React.createElement("div", {}, "Hello World");
  };
  window.extensionsAPI.registerResourceExtension(
    component,
    "*",
    "*",
    "Nice extension"
  );
})(window);
```

## System Level Extensions

Argo CD allows you to add new items to the sidebar that will be displayed as a new page with a custom component when clicked. The system level extension should be registered using the `extensionsAPI.registerSystemLevelExtension` method:

```typescript
registerSystemLevelExtension(component: ExtensionComponent, title: string, options: {icon?: string})
```

Below is an example of a simple system level extension:

```typescript
((window) => {
  const component = () => {
    return React.createElement(
      "div",
      { style: { padding: "10px" } },
      "Hello World"
    );
  };
  window.extensionsAPI.registerSystemLevelExtension(
    component,
    "Test Ext",
    "/hello",
    "fa-flask"
  );
})(window);
```

## Application Tab Extensions

Since the Argo CD Application is a Kubernetes resource, application tabs can be the same as any other resource tab.
Make sure to use 'argoproj.io'/'Application' as group/kind and an extension will be used to render the application-level tab.
