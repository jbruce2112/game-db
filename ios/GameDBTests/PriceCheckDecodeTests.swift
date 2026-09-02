import XCTest
@testable import GameDB

final class PriceCheckDecodeTests: XCTestCase {
    func testDecodesEbayPayloadWithNullCents() throws {
        let json = """
        {
          "title": "Halo 2",
          "platform": "Xbox",
          "barcode": "882224217927",
          "status": "ok",
          "value": {
            "pc_id": "ebay",
            "product_name": "Halo 2",
            "console_name": "Xbox",
            "url": "https://www.ebay.com/sch/139973/i.html?_nkw=Halo+2",
            "source": "ebay",
            "listings": 18,
            "loose_cents": 1499,
            "cib_cents": 2499,
            "new_cents": null
          }
        }
        """
        let got = try PriceCheckResult.decode(from: Data(json.utf8))
        XCTAssertEqual(got.status, "ok")
        XCTAssertEqual(got.value?.looseCents, 1499)
        XCTAssertEqual(got.value?.cibCents, 2499)
        XCTAssertNil(got.value?.newCents)
        XCTAssertEqual(got.value?.listings, 18)
        XCTAssertEqual(got.value?.source, "ebay")
    }

    func testDecodesNotFoundNullValue() throws {
        let json = """
        {"title":"Obscure","platform":"Atari 2600","status":"not_found","value":null}
        """
        let got = try PriceCheckResult.decode(from: Data(json.utf8))
        XCTAssertEqual(got.status, "not_found")
        XCTAssertNil(got.value)
    }

    func testDecodesNumericIdsAndOmittedListings() throws {
        let json = """
        {
          "title": "Sunshine",
          "platform": "Nintendo GameCube",
          "status": "ok",
          "value": {
            "pc_id": 3584,
            "product_name": "Super Mario Sunshine",
            "console_name": "Gamecube",
            "url": "",
            "loose_cents": "2495",
            "cib_cents": 5999.0
          }
        }
        """
        let got = try PriceCheckResult.decode(from: Data(json.utf8))
        XCTAssertEqual(got.value?.pcId, "3584")
        XCTAssertEqual(got.value?.looseCents, 2495)
        XCTAssertEqual(got.value?.cibCents, 5999)
    }

    func testHtmlBodyAsksForServerUpdate() {
        let html = Data("<!DOCTYPE html><html><body>Game Library</body></html>".utf8)
        XCTAssertThrowsError(try PriceCheckResult.decode(from: html)) { err in
            let message = (err as? APIError)?.errorDescription ?? ""
            XCTAssertTrue(message.contains("server update"), message)
        }
    }
}
