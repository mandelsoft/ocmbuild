# OCM Build Extension

This project provides a simple build feature to the [OCM](https://ocm.software) world. It offers to automate the way from a source project to an OCM transport archive containing the build artifacts generated from a source project. 

It is based on a *BuildFile.yaml*, which describes the intended component versions and the build steps required to generate the artifacts, which should be attached to the particular component version.

The build steps are executed by arbitrary *build plugins*, which are either consumed from an OCM repository or provided locally by previous build steps.
Every step may use a build plugin specific configuration, which describes:
- the build step itself
- and the resource(s) generated from this build step.

The described resources will then be added to the component version and after a successful build a transport archive is created for the described
component versions.

This content can then be transferred to a repository landscape.

## Provided build plugins

In addition to the framework,some build plugins are provided
- building a Go executable (`ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/goexecutable`)
- building a single/multiarch OCI images based on a Dockerfile and `docker`
  (`ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/dockerbuild`)
- describing any other resources with a constructor file similar to the `cm add cv` command  (`ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/constructor`)
- executing some command line (`ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/execute`)

## Example

This project is built by itself. This is achieved by using build plugin executables (which will later exposed and consumed by OCM component versions)
for the build steps. They are built on-the flay directly for their execution.

```yaml
schemaVersion: v1
metadata:
  # bootstrap defines a substitution of the used build plugins
  # by directly running them from the sources contained in
  # this project
  bootstrap:
    # plugin to execute some cpmmand
    execute:
      - go
      - run
      - gopkgpath: plugins/execute
    # plugin to build an Go executable
    goexecutable:
      - go
      - run
      - gopkgpath: plugins/goexecutable

  platforms:
    - linux/amd64
    - darwin/arm64

version: (( exec("go", "run", "ocm.software/ocm/api/version/generate", "print-rc-version") ))
provider:
  name: mandelsoft.org

builds:
  #  - pluginRef: ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/execute
  - executable: (( metadata.bootstrap.execute ))
    config:
      cmd:
        - go
        - test
        - gopkgpath: .

components:
  - name: ocm.software/plugins/ocmbuild
    builds:
      #      - pluginRef: ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/goexecutable
      - executable: (( metadata.bootstrap.goexecutable ))
        config:
          path: ocmplugin
          resource:
            name: ocmbuild
            type: ocmPlugin
          platforms: (( metadata.platforms ))

  - name: ocm.software/buildplugins/goexecutable
    builds:
      #      - pluginRef: ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/goexecutable
      - executable: (( metadata.bootstrap.goexecutable ))
        config:
          path: plugins/goexecutable
          resource:
            name: goexecutable
            type: ocm.software/buildplugin
          platforms: (( metadata.platforms ))

  - name: ocm.software/buildplugins/constructor
    builds:
      #      - pluginRef: ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/goexecutable
      - executable: (( metadata.bootstrap.goexecutable ))
        config:
          path: plugins/constructor
          resource:
            name: constructor
            type: ocm.software/buildplugin
          platforms: (( metadata.platforms ))

  - name: ocm.software/buildplugins/dockerbuild
    builds:
      #      - pluginRef: ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/goexecutable
      - executable: (( metadata.bootstrap.goexecutable ))
        config:
          path: plugins/dockerbuild
          resource:
            name: dockerbuild
            type: ocm.software/buildplugin
          platforms: (( metadata.platforms ))

  - name: ocm.software/buildplugins/execute
    builds:
      #      - pluginRef: ghcr.io/mandelsoft/ocmtest//ocm.software/buildplugins/goexecutable
      - executable: (( metadata.bootstrap.goexecutable ))
        config:
          path: plugins/execute
          resource:
            name: execute
            type: ocm.software/buildplugin
          platforms: (( metadata.platforms ))
```

## OCM Extension

The build tool can be used as standalone CLI tool, or as OCM plugin.
When installed into your OCM CLI environment
(`ocm install plugin ghcr.io/mandelsoft/ocmtest//ocm.software/plugins/ocmbuild`), it provides the command `ocm build componentversions`.