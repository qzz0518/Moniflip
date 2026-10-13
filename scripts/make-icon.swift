// Turns one square artwork PNG into the macOS-shaped app icon PNG that mygo
// packages (it only resizes assets/icon.png into the .icns and .ico).
//
// macOS icons are not bare squares: the artwork sits on Apple's rounded-rect
// grid — a 1024 pt canvas with an 824 pt body, continuous ("squircle") corners
// of radius 185.4, and transparent margins. SwiftUI's
// RoundedRectangle(style: .continuous) is that exact curve, so the mask is
// rendered rather than approximated.
//
//   swift scripts/make-icon.swift assets/icon-artwork.png assets/icon.png [docs/screenshots/icon.png]
//
// The optional third path gets a 512 px copy for the README.
import AppKit
import SwiftUI

let arguments = CommandLine.arguments
guard arguments.count >= 3 else {
    FileHandle.standardError.write(Data("usage: make-icon.swift <artwork.png> <icon.png> [readme.png]\n".utf8))
    exit(2)
}
let inputURL = URL(fileURLWithPath: arguments[1])

guard let artwork = NSImage(contentsOf: inputURL) else {
    FileHandle.standardError.write(Data("cannot read \(inputURL.path)\n".utf8))
    exit(1)
}

/// Apple's macOS icon grid, in points on a 1024 canvas.
enum Grid {
    static let canvas: CGFloat = 1024
    static let body: CGFloat = 824
    static let radius: CGFloat = 185.4
}

struct IconCanvas: View {
    let artwork: NSImage

    var body: some View {
        ZStack {
            Color.clear
            Image(nsImage: artwork)
                .resizable()
                .interpolation(.high)
                .aspectRatio(contentMode: .fill)
                .frame(width: Grid.body, height: Grid.body)
                .clipShape(RoundedRectangle(cornerRadius: Grid.radius, style: .continuous))
                // The shelf shadow in Apple's own template: soft, close, and far
                // weaker than a UI drop shadow.
                .shadow(color: .black.opacity(0.28), radius: 12, y: 10)
                .shadow(color: .black.opacity(0.12), radius: 3, y: 2)
        }
        .frame(width: Grid.canvas, height: Grid.canvas)
    }
}

@MainActor
func renderCanvas() -> CGImage? {
    let renderer = ImageRenderer(content: IconCanvas(artwork: artwork))
    renderer.scale = 1
    renderer.isOpaque = false
    return renderer.cgImage
}

func write(_ image: CGImage, side: Int, to url: URL) throws {
    let bitmap = NSBitmapImageRep(
        bitmapDataPlanes: nil, pixelsWide: side, pixelsHigh: side,
        bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
        colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0
    )!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
    NSGraphicsContext.current?.imageInterpolation = .high
    NSGraphicsContext.current?.cgContext.draw(image, in: CGRect(x: 0, y: 0, width: side, height: side))
    NSGraphicsContext.restoreGraphicsState()
    guard let data = bitmap.representation(using: .png, properties: [:]) else {
        throw CocoaError(.fileWriteUnknown)
    }
    try data.write(to: url)
}

guard let rendered = MainActor.assumeIsolated({ renderCanvas() }) else {
    FileHandle.standardError.write(Data("render failed\n".utf8))
    exit(1)
}
try write(rendered, side: 1024, to: URL(fileURLWithPath: arguments[2]))
print("wrote \(arguments[2])")
if arguments.count >= 4 {
    try write(rendered, side: 512, to: URL(fileURLWithPath: arguments[3]))
    print("wrote \(arguments[3])")
}
