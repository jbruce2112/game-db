import Foundation
import GRDB

struct PriceQuote: Codable, Equatable, Hashable {
    var pcId: String
    var productName: String
    var consoleName: String
    var url: String
    var looseCents: Int?
    var cibCents: Int?
    var newCents: Int?

    enum CodingKeys: String, CodingKey {
        case pcId = "pc_id"
        case productName = "product_name"
        case consoleName = "console_name"
        case url
        case looseCents = "loose_cents"
        case cibCents = "cib_cents"
        case newCents = "new_cents"
    }

    func cents(for completeness: String) -> Int? {
        switch completeness {
        case "loose": return looseCents
        case "cib": return cibCents
        case "new": return newCents
        default: return cibCents ?? looseCents ?? newCents
        }
    }

    static func usd(_ cents: Int) -> String {
        let v = Double(cents) / 100
        let f = NumberFormatter()
        f.numberStyle = .currency
        f.currencyCode = "USD"
        return f.string(from: NSNumber(value: v)) ?? String(format: "$%.2f", v)
    }
}
