import XCTest
@testable import GameDB

final class ShelfStatsTests: XCTestCase {
    func testCountsLiveItemsByPlatformRegionAndCompleteness() {
        var us = LibraryItem.newLocal(title: "Sunshine", platform: "GameCube", region: "us", completeness: "cib", notes: "")
        us.coverId = "abc"
        us.barcode = "123"
        let jp = LibraryItem.newLocal(title: "Pikmin", platform: "GameCube", region: "jp", completeness: "loose", notes: "")
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

    func testAskingPriceTotalsMedianAndTopCopies() {
        let mario = LibraryItem.newLocal(title: "Mario 64", platform: "Nintendo 64", region: "us", completeness: "cib", notes: "")
        let kart = LibraryItem.newLocal(title: "Mario Kart 64", platform: "Nintendo 64", region: "us", completeness: "loose", notes: "")
        let ico = LibraryItem.newLocal(title: "Ico", platform: "PlayStation 2", region: "us", completeness: "unknown", notes: "")
        let unpriced = LibraryItem.newLocal(title: "Homebrew", platform: "PlayStation 2", region: "us", completeness: "cib", notes: "")
        var gone = LibraryItem.newLocal(title: "Deleted", platform: "N64", region: "us", completeness: "cib", notes: "")
        gone.deletedAt = LibraryItem.now()

        let quotes: [String: PriceQuote] = [
            mario.id: quote(cib: 12_000, loose: 8_000),
            kart.id: quote(cib: 9_000, loose: 4_000),
            ico.id: quote(cib: 6_000, loose: 3_000),
            gone.id: quote(cib: 99_000, loose: 50_000),
        ]
        let stats = ShelfStats(items: [mario, kart, ico, unpriced, gone], quotes: quotes)
        XCTAssertEqual(stats.priced, 3)
        XCTAssertEqual(stats.shelfCents, 12_000 + 4_000 + 6_000)
        XCTAssertEqual(stats.medianCents, 6_000)
        XCTAssertEqual(stats.valueByPlatform.map(\.name), ["Nintendo 64", "PlayStation 2"])
        XCTAssertEqual(stats.valueByPlatform.map(\.cents), [16_000, 6_000])
        XCTAssertEqual(stats.mostExpensive.map(\.title), ["Mario 64", "Ico", "Mario Kart 64"])
        XCTAssertEqual(ShelfStats.median([1, 2, 3, 4]), 3)
    }

    func testCountsByYearFillsGapsAndFiltersPlatform() {
        var botw = LibraryItem.newLocal(title: "BotW", platform: "Nintendo Switch", region: "us", completeness: "cib", notes: "")
        botw.createdAt = "2023-11-12T00:00:00Z"
        var odyssey = LibraryItem.newLocal(title: "Odyssey", platform: "Nintendo Switch", region: "us", completeness: "cib", notes: "")
        odyssey.createdAt = "2023-11-12T00:00:00Z"
        var ico = LibraryItem.newLocal(title: "Ico", platform: "PlayStation 2", region: "us", completeness: "cib", notes: "")
        ico.createdAt = "2025-07-06T00:00:00Z"
        var gone = LibraryItem.newLocal(title: "Deleted", platform: "Nintendo Switch", region: "us", completeness: "cib", notes: "")
        gone.createdAt = "2024-01-01T00:00:00Z"
        gone.deletedAt = LibraryItem.now()

        let stats = ShelfStats(items: [botw, odyssey, ico, gone])
        XCTAssertEqual(stats.countsByYear(), [
            ShelfYearRow(year: 2023, count: 2),
            ShelfYearRow(year: 2024, count: 0),
            ShelfYearRow(year: 2025, count: 1),
        ])
        XCTAssertEqual(stats.countsByYear(platform: "Nintendo Switch"), [
            ShelfYearRow(year: 2023, count: 2),
        ])
        XCTAssertEqual(stats.cumulativeByYear(), [
            ShelfYearRow(year: 2023, count: 2),
            ShelfYearRow(year: 2024, count: 2),
            ShelfYearRow(year: 2025, count: 3),
        ])
        XCTAssertEqual(stats.cumulativeByYear(platform: "Nintendo Switch"), [
            ShelfYearRow(year: 2023, count: 2),
        ])
        XCTAssertEqual(ShelfStats.year(from: "2023-11-12T00:00:00Z"), 2023)
    }

    private func quote(cib: Int, loose: Int) -> PriceQuote {
        PriceQuote(
            pcId: "ebay",
            productName: "game",
            consoleName: "",
            url: "",
            source: "ebay",
            listings: 3,
            looseCents: loose,
            cibCents: cib,
            newCents: nil
        )
    }
}
