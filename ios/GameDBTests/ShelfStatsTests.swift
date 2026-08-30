import XCTest
@testable import GameDB

final class ShelfStatsTests: XCTestCase {
    func testCountsLiveItemsByPlatformRegionAndCompleteness() {
        var us = LibraryItem.newLocal(title: "Sunshine", platform: "GameCube", region: "us", completeness: "cib", notes: "")
        us.coverId = "abc"
        us.barcode = "123"
        var jp = LibraryItem.newLocal(title: "Pikmin", platform: "GameCube", region: "jp", completeness: "loose", notes: "")
        var gone = LibraryItem.newLocal(title: "Deleted", platform: "N64", region: "us", completeness: "new", notes: "")
        gone.deletedAt = LibraryItem.now()
        let ps = LibraryItem.newLocal(title: "Ico", platform: "PlayStation 2", region: nil, completeness: "unknown", notes: "")

        let stats = ShelfStats(items: [us, jp, gone, ps])
        XCTAssertEqual(stats.total, 3)
        XCTAssertEqual(stats.withCover, 1)
        XCTAssertEqual(stats.withBarcode, 1)
        XCTAssertEqual(stats.platforms, [
            ShelfRow(name: "GameCube", count: 2),
            ShelfRow(name: "PlayStation 2", count: 1),
        ])
        XCTAssertEqual(stats.regions.map(\.name), ["USA", "Japan", "Unset"])
        XCTAssertEqual(stats.completeness.map(\.name), ["CIB", "Loose", "Unknown"])
        XCTAssertEqual(ShelfStats.percent(1, of: 3), "33%")
    }

    func testEmptyLibrary() {
        XCTAssertEqual(ShelfStats(items: []), .empty)
        XCTAssertEqual(ShelfStats.percent(0, of: 0), "0%")
    }
}
