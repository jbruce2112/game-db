import Foundation

struct ShelfRow: Equatable {
    var name: String
    var count: Int
}

struct ShelfValueRow: Equatable {
    var name: String
    var cents: Int
}

struct ShelfPriceRow: Equatable {
    var id: String
    var title: String
    var platform: String
    var cents: Int
}

struct ShelfYearRow: Equatable {
    var year: Int
    var count: Int
}

struct ShelfYearAdd: Equatable {
    var year: Int
    var platform: String
}

struct ShelfStats: Equatable {
    var total: Int
    var withCover: Int
    var withBarcode: Int
    var priced: Int
    var shelfCents: Int?
    var medianCents: Int?
    var platforms: [ShelfRow]
    var regions: [ShelfRow]
    var completeness: [ShelfRow]
    var valueByPlatform: [ShelfValueRow]
    var mostExpensive: [ShelfPriceRow]
    var yearAdds: [ShelfYearAdd]

    static let empty = ShelfStats(items: [])

    static func percent(_ n: Int, of total: Int) -> String {
        guard total > 0 else { return "0%" }
        return "\(Int((Double(n) / Double(total) * 100).rounded()))%"
    }

    static func median(_ values: [Int]) -> Int? {
        guard !values.isEmpty else { return nil }
        let sorted = values.sorted()
        let mid = sorted.count / 2
        if sorted.count % 2 == 1 {
            return sorted[mid]
        }
        return Int((Double(sorted[mid - 1] + sorted[mid]) / 2).rounded())
    }

    init(items: [LibraryItem], quotes: [String: PriceQuote] = [:]) {
        let live = items.filter { ($0.deletedAt ?? "").isEmpty }
        total = live.count
        withCover = live.filter { !($0.coverId ?? "").isEmpty }.count
        withBarcode = live.filter { !($0.barcode ?? "").isEmpty }.count

        var plat: [String: Int] = [:]
        var region: [String: Int] = [:]
        var complete: [String: Int] = [:]
        var platValue: [String: Int] = [:]
        var pricedRows: [ShelfPriceRow] = []
        for item in live {
            plat[item.platform, default: 0] += 1
            let r = item.region ?? ""
            region[r, default: 0] += 1
            let c = item.completeness.isEmpty ? "unknown" : item.completeness
            complete[c, default: 0] += 1
            if let cents = quotes[item.id]?.cents(for: item.completeness) {
                platValue[item.platform, default: 0] += cents
                pricedRows.append(ShelfPriceRow(id: item.id, title: item.title, platform: item.platform, cents: cents))
            }
        }
        priced = pricedRows.count
        if priced > 0 {
            shelfCents = pricedRows.reduce(0) { $0 + $1.cents }
            medianCents = Self.median(pricedRows.map(\.cents))
        } else {
            shelfCents = nil
            medianCents = nil
        }
        platforms = plat.keys.sorted { (plat[$0] ?? 0) > (plat[$1] ?? 0) || ((plat[$0] ?? 0) == (plat[$1] ?? 0) && $0 < $1) }
            .map { ShelfRow(name: $0, count: plat[$0] ?? 0) }
        let regionOrder = ["us", "eu", "jp", "au", "other", ""]
        let regionLabels = ["us": "USA", "eu": "Europe", "jp": "Japan", "au": "Australia", "other": "Other", "": "Unset"]
        regions = regionOrder.compactMap { key in
            guard let n = region[key], n > 0 else { return nil }
            return ShelfRow(name: regionLabels[key] ?? key, count: n)
        }
        let completeOrder = ["cib", "loose", "new", "unknown"]
        let completeLabels = ["cib": "CIB", "loose": "Loose", "new": "New", "unknown": "Unknown"]
        completeness = completeOrder.compactMap { key in
            guard let n = complete[key], n > 0 else { return nil }
            return ShelfRow(name: completeLabels[key] ?? key, count: n)
        }
        valueByPlatform = platValue.keys
            .filter { (platValue[$0] ?? 0) > 0 }
            .sorted { (platValue[$0] ?? 0) > (platValue[$1] ?? 0) || ((platValue[$0] ?? 0) == (platValue[$1] ?? 0) && $0 < $1) }
            .map { ShelfValueRow(name: $0, cents: platValue[$0] ?? 0) }
        mostExpensive = Array(pricedRows.sorted { $0.cents > $1.cents || ($0.cents == $1.cents && $0.title < $1.title) }.prefix(10))
        yearAdds = live.compactMap { item in
            guard let year = Self.year(from: item.createdAt) else { return nil }
            return ShelfYearAdd(year: year, platform: item.platform)
        }
    }

    static func year(from iso: String) -> Int? {
        guard iso.count >= 4, let y = Int(iso.prefix(4)), (1970...2100).contains(y) else { return nil }
        return y
    }

    func countsByYear(platform: String = "") -> [ShelfYearRow] {
        let rows = platform.isEmpty ? yearAdds : yearAdds.filter { $0.platform == platform }
        guard !rows.isEmpty else { return [] }
        var map: [Int: Int] = [:]
        var minY = Int.max
        var maxY = Int.min
        for row in rows {
            map[row.year, default: 0] += 1
            minY = min(minY, row.year)
            maxY = max(maxY, row.year)
        }
        return (minY...maxY).map { ShelfYearRow(year: $0, count: map[$0] ?? 0) }
    }

    func cumulativeByYear(platform: String = "") -> [ShelfYearRow] {
        var sum = 0
        return countsByYear(platform: platform).map { row in
            sum += row.count
            return ShelfYearRow(year: row.year, count: sum)
        }
    }
}
