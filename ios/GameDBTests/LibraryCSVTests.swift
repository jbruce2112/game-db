import XCTest
@testable import GameDB

final class LibraryCSVTests: XCTestCase {
    func testParsesCLZAddedDate() throws {
        let raw = """
        Platform,Title,"Release Date",Publisher,Developer,Genre,"Added Date",Barcode,Region
        "PlayStation 4","Shin Megami Tensei III Nocturne HD Remaster","May 25, 2021",Atlus,Atlus,RPG,"Aug 29, 2026",730865220366,USA
        """
        let items = try LibraryCSV.parse(raw)
        XCTAssertEqual(items.count, 1)
        XCTAssertEqual(RFC3339.addedLabel(items[0].createdAt), "Aug 29, 2026")
        XCTAssertTrue(items[0].createdAt.hasPrefix("2026-08-29"))
    }

    func testAddedLabelKeepsUTCCalendarDay() {
        XCTAssertEqual(RFC3339.addedLabel("2026-08-29T00:00:00Z"), "Aug 29, 2026")
    }
}
