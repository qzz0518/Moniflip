#!/usr/bin/env swift
// Moniflip 的 DMG 背景：从 assets/dmg/background-source.png 裁成 660x400，
// 画上标题、副标题、箭头和图标名底板（沿用 Charker 的 make-dmg-background.swift）。
// 1x、2x 各画一张，用 tiffutil 合成多分辨率 TIFF，Retina 屏上 Finder 取 2x。
//
//   swift scripts/make-dmg-background.swift assets/dmg/background-source.png assets/dmg/background.tiff
import AppKit
import Foundation

private let arguments = CommandLine.arguments
guard arguments.count == 3 else {
    fputs("usage: make-dmg-background.swift <source.png> <output.tiff>\n", stderr)
    exit(2)
}

let sourceURL = URL(fileURLWithPath: arguments[1])
let outputURL = URL(fileURLWithPath: arguments[2])
guard let source = NSImage(contentsOf: sourceURL) else {
    fputs("cannot read \(sourceURL.path)\n", stderr)
    exit(1)
}

let size = NSSize(width: 660, height: 400)
let targetAspect = size.width / size.height
let sourceAspect = source.size.width / source.size.height
let sourceRect: NSRect
if sourceAspect > targetAspect {
    let width = source.size.height * targetAspect
    sourceRect = NSRect(
        x: (source.size.width - width) / 2,
        y: 0,
        width: width,
        height: source.size.height
    )
} else {
    let height = source.size.width / targetAspect
    sourceRect = NSRect(
        x: 0,
        y: (source.size.height - height) / 2,
        width: source.size.width,
        height: height
    )
}

// render 按点画出背景，位图像素是点数的 scale 倍。
func render(scale: Int) -> Data {
    guard let bitmap = NSBitmapImageRep(
        bitmapDataPlanes: nil,
        pixelsWide: Int(size.width) * scale,
        pixelsHigh: Int(size.height) * scale,
        bitsPerSample: 8,
        samplesPerPixel: 4,
        hasAlpha: true,
        isPlanar: false,
        colorSpaceName: .deviceRGB,
        bytesPerRow: 0,
        bitsPerPixel: 0
    ) else {
        fputs("cannot create DMG background canvas\n", stderr)
        exit(1)
    }
    // 先定点数再建上下文，上下文才会按 scale 缩放。
    bitmap.size = size
    guard let context = NSGraphicsContext(bitmapImageRep: bitmap) else {
        fputs("cannot create DMG background canvas\n", stderr)
        exit(1)
    }
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = context
    context.imageInterpolation = .high
    source.draw(
        in: NSRect(origin: .zero, size: size),
        from: sourceRect,
        operation: .copy,
        fraction: 1
    )

    // Finder chooses black icon-label text even when the background image is dark.
    // Quiet light backplates keep both native labels readable without baking a
    // second copy of their text into the artwork.
    for rect in [
        NSRect(x: 109, y: 88, width: 138, height: 28),
        NSRect(x: 417, y: 88, width: 130, height: 28),
    ] {
        let plate = NSBezierPath(roundedRect: rect, xRadius: 9, yRadius: 9)
        NSColor(calibratedRed: 0.55, green: 0.60, blue: 0.67, alpha: 0.92).setFill()
        plate.fill()
        NSColor.white.withAlphaComponent(0.16).setStroke()
        plate.lineWidth = 0.75
        plate.stroke()
    }

    let centered = NSMutableParagraphStyle()
    centered.alignment = .center

    let title = "拖动安装  ·  Drag to install"
    let titleAttributes: [NSAttributedString.Key: Any] = [
        .font: NSFont.systemFont(ofSize: 21, weight: .semibold),
        .foregroundColor: NSColor(calibratedWhite: 0.96, alpha: 1),
        .paragraphStyle: centered,
        .kern: 0.2,
    ]
    title.draw(
        in: NSRect(x: 40, y: 329, width: 580, height: 30),
        withAttributes: titleAttributes
    )

    let subtitle = "将 Moniflip 拖到“应用程序”文件夹  ·  Drag Moniflip to the Applications folder"
    let subtitleAttributes: [NSAttributedString.Key: Any] = [
        .font: NSFont.systemFont(ofSize: 12, weight: .medium),
        .foregroundColor: NSColor(calibratedRed: 0.57, green: 0.62, blue: 0.70, alpha: 1),
        .paragraphStyle: centered,
        .kern: 0.1,
    ]
    subtitle.draw(
        in: NSRect(x: 40, y: 306, width: 580, height: 20),
        withAttributes: subtitleAttributes
    )

    let accent = NSColor(calibratedRed: 0.62, green: 0.70, blue: 1.0, alpha: 0.95)
    let arrow = NSBezierPath()
    arrow.move(to: NSPoint(x: 268, y: 166))
    arrow.line(to: NSPoint(x: 389, y: 166))
    arrow.move(to: NSPoint(x: 389, y: 166))
    arrow.line(to: NSPoint(x: 376, y: 174))
    arrow.move(to: NSPoint(x: 389, y: 166))
    arrow.line(to: NSPoint(x: 376, y: 158))
    arrow.lineWidth = 2
    arrow.lineCapStyle = .round
    arrow.lineJoinStyle = .round
    accent.setStroke()
    arrow.stroke()

    let startDot = NSBezierPath(ovalIn: NSRect(x: 262, y: 163, width: 6, height: 6))
    accent.withAlphaComponent(0.72).setFill()
    startDot.fill()
    NSGraphicsContext.restoreGraphicsState()

    guard let png = bitmap.representation(using: .png, properties: [:]) else {
        fputs("cannot encode DMG background\n", stderr)
        exit(1)
    }
    return png
}

let work = FileManager.default.temporaryDirectory
    .appendingPathComponent("moniflip-dmg-background-\(getpid())")
try FileManager.default.createDirectory(at: work, withIntermediateDirectories: true)
defer { try? FileManager.default.removeItem(at: work) }
let pngs = [1, 2].map { work.appendingPathComponent("background@\($0)x.png") }
try render(scale: 1).write(to: pngs[0])
try render(scale: 2).write(to: pngs[1])

let tiffutil = Process()
tiffutil.executableURL = URL(fileURLWithPath: "/usr/bin/tiffutil")
tiffutil.arguments = ["-cathidpicheck", pngs[0].path, pngs[1].path, "-out", outputURL.path]
try tiffutil.run()
tiffutil.waitUntilExit()
guard tiffutil.terminationStatus == 0 else {
    fputs("tiffutil failed\n", stderr)
    exit(1)
}
print("wrote \(outputURL.path) (\(Int(size.width))x\(Int(size.height)) @1x, @2x)")
