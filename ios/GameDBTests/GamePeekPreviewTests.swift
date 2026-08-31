import SwiftUI
import UIKit
import XCTest
@testable import GameDB

final class GamePeekPreviewTests: XCTestCase {
    /// Peek previews render in an isolated hosting controller. They must not
    /// read `@Environment(LibraryStore.self)` — that lookup fatals when the
    /// store is missing (the original long-press crash).
    @MainActor
    func testPeekPreviewHostsWithoutLibraryStore() {
        let item = LibraryItem.newLocal(
            title: "Sunshine",
            platform: "GameCube",
            region: "NTSC",
            completeness: "cib",
            notes: ""
        )
        hostAndLayout(GamePeekPreview(item: item, image: nil))
    }

    @MainActor
    func testPeekPreviewLoadsCoverFromDisk() throws {
        let coverId = UUID().uuidString.lowercased()
        let url = AppDatabase.coverURL(id: coverId)
        let jpeg = try XCTUnwrap(
            UIGraphicsImageRenderer(size: CGSize(width: 16, height: 16)).image { ctx in
                UIColor.blue.setFill()
                ctx.fill(CGRect(x: 0, y: 0, width: 16, height: 16))
            }.jpegData(compressionQuality: 0.9)
        )
        try jpeg.write(to: url, options: .atomic)
        defer { try? FileManager.default.removeItem(at: url) }

        XCTAssertNotNil(CoverFile.image(for: coverId))

        var item = LibraryItem.newLocal(
            title: "Sunshine",
            platform: "GameCube",
            region: nil,
            completeness: "cib",
            notes: ""
        )
        item.coverId = coverId
        hostAndLayout(GamePeekPreview(item: item, image: nil))
    }

    @MainActor
    func testPeekPreviewHostsWithCoverImage() {
        let item = LibraryItem.newLocal(
            title: "Sunshine",
            platform: "GameCube",
            region: nil,
            completeness: "unknown",
            notes: ""
        )
        let image = UIGraphicsImageRenderer(size: CGSize(width: 8, height: 8)).image { ctx in
            UIColor.red.setFill()
            ctx.fill(CGRect(x: 0, y: 0, width: 8, height: 8))
        }
        hostAndLayout(GamePeekPreview(item: item, image: image))
    }

    @MainActor
    func testPeekPreviewHostsLandscapeBoxArt() {
        let item = LibraryItem.newLocal(
            title: "Mario 64",
            platform: "Nintendo 64",
            region: "us",
            completeness: "cib",
            notes: ""
        )
        let image = UIGraphicsImageRenderer(size: CGSize(width: 32, height: 20)).image { ctx in
            UIColor.green.setFill()
            ctx.fill(CGRect(x: 0, y: 0, width: 32, height: 20))
        }
        hostAndLayout(GamePeekPreview(item: item, image: image))
    }

    @MainActor
    private func hostAndLayout<V: View>(_ root: V) {
        let host = UIHostingController(rootView: root)
        let window = UIWindow(frame: CGRect(x: 0, y: 0, width: 390, height: 844))
        window.rootViewController = host
        window.makeKeyAndVisible()
        host.loadViewIfNeeded()
        host.view.setNeedsLayout()
        host.view.layoutIfNeeded()
        XCTAssertNotNil(host.view)
        XCTAssertGreaterThan(host.view.bounds.width, 0)
        // Keep the window alive through the layout pass.
        _ = window
    }
}
