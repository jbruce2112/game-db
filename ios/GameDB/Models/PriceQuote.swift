import Foundation
import GRDB

struct PriceQuote: Codable, Equatable, Hashable {
    var pcId: String
    var productName: String
    var consoleName: String
    var url: String
    var source: String?
    var listings: Int?
    var looseCents: Int?
    var cibCents: Int?
    var newCents: Int?

    enum CodingKeys: String, CodingKey {
        case pcId = "pc_id"
        case productName = "product_name"
        case consoleName = "console_name"
        case url, source, listings
        case looseCents = "loose_cents"
        case cibCents = "cib_cents"
        case newCents = "new_cents"
    }

    func cents(for completeness: String) -> Int? {
        switch completeness {
        case "loose": return looseCents ?? cibCents
        case "cib": return cibCents ?? looseCents ?? newCents
        case "new": return newCents ?? cibCents ?? looseCents
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

extension PriceQuote {
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        pcId = c.decodeFlexibleString(.pcId)
        productName = c.decodeFlexibleString(.productName)
        consoleName = c.decodeFlexibleString(.consoleName)
        url = c.decodeFlexibleString(.url)
        let src = c.decodeFlexibleString(.source)
        source = src.isEmpty ? nil : src
        listings = c.decodeFlexibleInt(.listings)
        looseCents = c.decodeFlexibleInt(.looseCents)
        cibCents = c.decodeFlexibleInt(.cibCents)
        newCents = c.decodeFlexibleInt(.newCents)
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(pcId, forKey: .pcId)
        try c.encode(productName, forKey: .productName)
        try c.encode(consoleName, forKey: .consoleName)
        try c.encode(url, forKey: .url)
        try c.encodeIfPresent(source, forKey: .source)
        try c.encodeIfPresent(listings, forKey: .listings)
        try c.encodeIfPresent(looseCents, forKey: .looseCents)
        try c.encodeIfPresent(cibCents, forKey: .cibCents)
        try c.encodeIfPresent(newCents, forKey: .newCents)
    }
}

struct PriceCheckResult: Codable {
    var title: String
    var platform: String
    var barcode: String?
    var status: String
    var value: PriceQuote?

    enum CodingKeys: String, CodingKey {
        case title, platform, barcode, status, value
    }

    init(title: String, platform: String, barcode: String?, status: String, value: PriceQuote?) {
        self.title = title
        self.platform = platform
        self.barcode = barcode
        self.status = status
        self.value = value
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        title = c.decodeFlexibleString(.title)
        platform = c.decodeFlexibleString(.platform)
        let code = c.decodeFlexibleString(.barcode)
        barcode = code.isEmpty ? nil : code
        let st = c.decodeFlexibleString(.status)
        status = st.isEmpty ? "not_found" : st
        value = try? c.decodeIfPresent(PriceQuote.self, forKey: .value)
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(title, forKey: .title)
        try c.encode(platform, forKey: .platform)
        try c.encodeIfPresent(barcode, forKey: .barcode)
        try c.encode(status, forKey: .status)
        try c.encodeIfPresent(value, forKey: .value)
    }

    static func decode(from data: Data) throws -> PriceCheckResult {
        let trimmed = String(data: data, encoding: .utf8)?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if trimmed.hasPrefix("<") || trimmed.lowercased().contains("<html") {
            throw APIError.server("Price check needs a server update. Rebuild and restart game-db.")
        }
        do {
            return try JSONDecoder().decode(PriceCheckResult.self, from: data)
        } catch let err as DecodingError {
            throw APIError.server(describeDecoding(err))
        }
    }
}

private func describeDecoding(_ error: DecodingError) -> String {
    switch error {
    case .keyNotFound(let key, let ctx):
        let path = (ctx.codingPath + [key]).map(\.stringValue).joined(separator: ".")
        return "Price data was missing \(path)."
    case .typeMismatch(_, let ctx), .valueNotFound(_, let ctx):
        let path = ctx.codingPath.map(\.stringValue).joined(separator: ".")
        return "Price data was invalid\(path.isEmpty ? "" : " for \(path)")."
    default:
        return "Could not read prices from the server."
    }
}

private extension KeyedDecodingContainer {
    func decodeFlexibleString(_ key: Key) -> String {
        if let s = try? decodeIfPresent(String.self, forKey: key) {
            return s
        }
        if let i = try? decodeIfPresent(Int.self, forKey: key) {
            return String(i)
        }
        if let d = try? decodeIfPresent(Double.self, forKey: key) {
            return String(d)
        }
        return ""
    }

    func decodeFlexibleInt(_ key: Key) -> Int? {
        if let i = try? decodeIfPresent(Int.self, forKey: key) {
            return i
        }
        if let d = try? decodeIfPresent(Double.self, forKey: key) {
            return Int(d.rounded())
        }
        if let s = try? decodeIfPresent(String.self, forKey: key), let i = Int(s) {
            return i
        }
        return nil
    }
}
