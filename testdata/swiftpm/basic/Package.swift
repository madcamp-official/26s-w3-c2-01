// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "MyLibrary",
    products: [
        .library(name: "MyLibrary", targets: ["MyLibrary"]),
    ],
    targets: [
        .target(name: "MyLibrary"),
    ]
)
