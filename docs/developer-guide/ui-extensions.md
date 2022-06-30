# UI Extensions

Argo CD web user interface can be extended with additional UI elements. Extensions should be delivered as a javascript file
in the `argocd-server` Pods that are placed in the `/tmp/extensions` directory and starts with `extension` prefix ( matches to `^extension(.*).js$` regex ).

```
/tmp/extensions
├── example1
│   └── extension-1.js
└── example2
    └── extension-2.js
```

Extensions are loaded during initial page rendering and should register themselves using API exposed in the `extensionsAPI` global variable. (See
corresponding extention type details for additional information).

The extension should provide a React component that is responsible for rendering the UI element. Extension should not bundle the React library.
Instead extension should use the `react` global variable. You can leverage `externals` setting if you are using webpack:

```js
  externals: {
    react: 'React'
  }
```

## Resource Tab Extensions

Resource Tab extensions is an extension that provides an additional tab for the resource sliding panel at the Argo CD Application details page.

<img width="568" alt="image" src="https://user-images.githubusercontent.com/426437/176794114-cb3707a4-1d65-4468-91d7-4fd54e9e9d42.png">

The resource tab extension should be registered using the `extensionsAPI.registerResourceExtension` method:

```typescript
registerResourceExtension(component: ExtensionComponent, group: string, kind: string, tabTitle: string)
```



* `component: ExtensionComponent` is a React component that receives the following properties:

    * resource: State - the kubernetes resource object;
    * tree: ApplicationTree - includes list of all resources that comprise the application;

    See properties interfaces in [models.ts](https://github.com/argoproj/argo-cd/blob/master/ui/src/app/shared/models.ts)

* `group: string` - the glob expression that matches the group of the resource;
* `kind: string` - the glob expression that matches the kind of the resource;
* `tabTitle: string` - the extension tab title.

Below is an example of a resource tab extension:

```javascript
((window) => {
    const component = () => {
        return React.createElement( 'div', {}, 'Hello World' );
    };
    window.extensionsAPI.registerResourceExtension(component, '*', '*', 'Nice extension');
})(window)
```

