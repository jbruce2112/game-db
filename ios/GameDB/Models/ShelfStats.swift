import Foundation

struct ShelfRow: Equatable {
    var name: String
    var count: Int
}

struct ShelfStats: Equatable {
    var total: Int
    var withCover: Int
    var withBarcode: Int
    var platforms: [ShelfRow]
    var regions: [ShelfRow]
    var completeness: [ShelfRow]

    static let empty = ShelfStats(items: [])

    static func percent(_ n: Int, of total: Int) -> String {
        guard total > 0 else { return "0%" }
        return "\(Int((Double(n) / Double(total) * 100).rounded()))%"
    }

    init(items: [LibraryItem]) {
        let live = items.filter { ($0.deletedAt ?? "").isEmpty }
        total = live.count
        withCover = live.filter { !($0.coverId ?? "").isEmpty }.count
        withBarcode = live.filter { !($0.barcode ?? "").isEmpty }.count

        var plat: [String: Int] = [:]
        var region: [String: Int] = [:]
        var complete: [String: Int] = [:]
        for item in live {
            plat[item.platform, default: 0] += 1
            let r = item.region ?? ""
            region[r, default: 0] += 1
            let c = item.completeness.isEmpty ? "unknown" : item.completeness
            complete[c, default: 0] += 1
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
    }
}
