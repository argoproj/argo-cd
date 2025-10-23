The Argo CD UI displays icons for various Kubernetes resource types to help users quickly identify them. Argo CD 
includes a set of built-in icons for common resource types.

You can contribute additional icons for custom resource types by following these steps:

1. Ensure the license is compatible with Apache 2.0.
2. Add the icon file to the `ui/src/assets/images/resources/<group>/icon.svg` path in the Argo CD repository.
3. Modify the SVG to use the correct color, `#8fa4b1`.
4. Run `make resourceiconsgen` to update the generated typescript file that lists all available icons.
5. Create a pull request to the Argo CD repository with your changes.

`<group>` is the API group of the custom resource. For example, if you are adding an icon for a custom resource with the 
API group `example.com`, you would place the icon at `ui/src/assets/images/resources/example.com/icon.svg`.

If you want the same icon to apply to resources in multiple API groups with the same suffix, you can create a directory
prefixed with an underscore. The underscore will be interpreted as a wildcard. For example, to apply the same icon to
resources in the `example.com` and `another.example.com` API groups, you would place the icon at
`ui/src/assets/images/resources/_.example.com/icon.svg`.
