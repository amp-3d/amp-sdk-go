# AMP / Arc Media Platform

[![Build Status](https://github.com/amp-3d/amp-sdk-go/workflows/go/badge.svg?branch=main)](https://github.com/amp-3d/amp-sdk-go/actions)

_A turn-key solution for creating high-fidelity, multi-platform 3D experiences_

## What is AMP?

This repo contains Go interfaces and basic tools for the _Arc Media Platform_ ("AMP"), a SDK for building multi-platform 3D and media-centric apps with pluggable infrastructure. The AMP client is a [Unity](https://unity.com) based app driven by an embedded [Go](https://golang.org) runtime.  This means AMP is a "turn-key" 3D-based user interface solution allowing you rapidly publish a native app on Windows, macOS, Linux, Android, iOS, and most XR headsets while also delivering the benefits of Go.

This is interesting to:
  - **Storage and content providers** can deploy visually stunning user experiences using AMP, delivering an immersive media experience while attracting users.  This is otherwise only available in AAA games and profoudly more painful through a pure web-based solution -- e.g. [IPFS](https://www.ipfs.com/), Amazon's [S3](https://aws.amazon.com/s3/).
  - **Library and app developers** regard AMP as a flexible UI framework that allows them to focus on their core value proposition. Easily add your own 2D or 3D custom UI components while you get multi-platform build support out of the box.  E.g. data visualization, geographical and spatial linking.
  - **Geo/Spatial workspaces** are common in geo/spatial centric applications, such as GIS, CAD, and BIM, where integrated 3D visualization is a core part of the user experience.  AMP's Unity client natively integrates [Cesium](https://cesium.com/) mapping, allowing you to unify location-based datasets, spatially precise environments, high-fidelity 3D rendering, and extensible linking.

This repo is lightweight and dependency-free so that it can be added to your project without consequence.  At a high level, the development workflow is:

  1. Import [amp-sdk-go](https://github.com/amp-3d/amp-sdk-go) in your Go project and expose your functionally as an `amp.App`.
  2. Clone [amp-host-go](https://github.com/amp-3d/amp-host-go) and embed and expose your Go packages within it.
  3. `make build libarchost` (with your packages embedded within it).
  4. Rapidly build a native Unity app using one of the AMP "app" templates, embedding `libarchost` within it.
  5. At runtime, the Unity client any root `amp.Cell` of your app is "pinned" via `amp://{yourAppTagID}/{yourSchema...}` while the AMP UX runtime manages the user's perceptual experience of all actively pinned URIs.

## Points of Interest

In suggested order of review for newcomers:

|                          |                                                                   |
|------------------------- | ------------------------------------------------------------------|
| [api.task.go](https://github.com/amp-3d/amp-sdk-go/blob/main/stdlib/task/api.task.go)        | A wrapper for goroutines inspired by a conventional parent-child process model and is used throughout this SDK.              |
| [api.app.go](https://github.com/amp-3d/amp-sdk-go/blob/main/amp/api.app.go)    | Interfaces for developers looking to implement an `amp.App`, defining how state is requested, pushed, and merged.                |
| [api.host.go](https://github.com/amp-3d/amp-sdk-go/blob/main/amp/api.host.go) | Defines `amp.Host` and its related types, what [`amp-host-go`](https://github.com/amp-3d/amp-host-go) implements, and the abstraction that an `amp.App` plugs into.             |

## What is `amp.App`?

[`amp.App`](https://github.com/amp-3d/amp-sdk-go/blob/main/amp/api.app.go) is the plugin interface for AMP.

Like a traditional OS service, an `amp.App` responds to queries it recognizes and operates on user data and system state.   The stock AMP runtime offers essential apps, such as file system access, and user account services.  The less obvious power of AMP its extensibility. This is done by implementing the `amp.App` interface and registering it with the Go-based AMP runtime.  The AMP runtime then manages the user's perceptual experience of all actively pinned cells on an AMP-compatible client.
