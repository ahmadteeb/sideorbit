// swift-tools-version: 5.9
import PackageDescription

// macOS is listed so CI can run these tests with `swift test`, without a simulator.
let package = Package(
    name: "OrbitKit",
    platforms: [.iOS(.v16), .macOS(.v13)],
    products: [.library(name: "OrbitKit", targets: ["OrbitKit"])],
    targets: [
        .target(name: "OrbitKit"),
        .testTarget(name: "OrbitKitTests", dependencies: ["OrbitKit"]),
    ]
)
