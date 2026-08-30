import Foundation

enum LibraryCSV {
    static let header = [
        "id", "title", "platform", "region", "completeness", "notes",
        "igdb_game_id", "igdb_platform_id", "barcode", "created_at", "updated_at",
    ]

    static func string(from items: [LibraryItem]) -> String {
        var lines = [header.map(escape).joined(separator: ",")]
        for it in items {
            let row = [
                it.id,
                it.title,
                it.platform,
                it.region ?? "",
                it.completeness,
                it.notes,
                it.igdbGameId.map(String.init) ?? "",
                it.igdbPlatformId.map(String.init) ?? "",
                it.barcode ?? "",
                it.createdAt,
                it.updatedAt,
            ]
            lines.append(row.map(escape).joined(separator: ","))
        }
        return "\u{FEFF}" + lines.joined(separator: "\r\n") + "\r\n"
    }

    static func fileURL(from items: [LibraryItem]) throws -> URL {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd"
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = TimeZone(secondsFromGMT: 0)
        let name = "game-db-\(formatter.string(from: Date())).csv"
        let url = FileManager.default.temporaryDirectory.appendingPathComponent(name)
        try string(from: items).data(using: .utf8)!.write(to: url, options: .atomic)
        return url
    }

    private static func escape(_ field: String) -> String {
        if field.contains(where: { $0 == "," || $0 == "\"" || $0 == "\n" || $0 == "\r" }) {
            return "\"\(field.replacingOccurrences(of: "\"", with: "\"\""))\""
        }
        return field
    }

    static func parse(_ raw: String) throws -> [LibraryItem] {
        var text = raw
        if text.hasPrefix("\u{FEFF}") {
            text.removeFirst()
        }
        let rows = try records(from: text)
        guard let header = rows.first else { throw CSVError.empty }
        var idx: [String: Int] = [:]
        for (i, name) in header.enumerated() {
            idx[name.lowercased().trimmingCharacters(in: .whitespaces)] = i
        }
        guard idx["title"] != nil, idx["platform"] != nil else {
            throw CSVError.missingColumns
        }
        func get(_ row: [String], _ key: String) -> String {
            guard let i = idx[key], i < row.count else { return "" }
            return row[i].trimmingCharacters(in: .whitespaces)
        }
        var seen = Set<String>()
        var items: [LibraryItem] = []
        let now = LibraryItem.now()
        for row in rows.dropFirst() {
            let title = get(row, "title")
            let platform = get(row, "platform")
            if title.isEmpty && platform.isEmpty && get(row, "id").isEmpty { continue }
            if title.isEmpty || platform.isEmpty { throw CSVError.missingTitlePlatform }
            var id = get(row, "id").lowercased()
            if id.count != 36 { id = UUID().uuidString.lowercased() }
            if seen.contains(id) { throw CSVError.duplicateID }
            seen.insert(id)
            let created = get(row, "created_at")
            let updated = get(row, "updated_at")
            items.append(LibraryItem(
                id: id,
                title: title,
                platform: platform,
                igdbPlatformId: Int64(get(row, "igdb_platform_id")),
                region: {
                    let r = get(row, "region").lowercased()
                    return ["us", "eu", "jp", "au", "other"].contains(r) ? r : nil
                }(),
                completeness: {
                    let c = get(row, "completeness").lowercased()
                    return ["loose", "cib", "new"].contains(c) ? c : "unknown"
                }(),
                notes: get(row, "notes"),
                igdbGameId: Int64(get(row, "igdb_game_id")),
                coverId: nil,
                barcode: {
                    let b = get(row, "barcode").filter(\.isNumber)
                    return b.isEmpty ? nil : b
                }(),
                createdAt: created.isEmpty ? now : created,
                updatedAt: updated.isEmpty ? (created.isEmpty ? now : created) : updated,
                deletedAt: nil,
                syncSeq: 0,
                dirty: true,
                value: nil
            ))
        }
        if items.isEmpty { throw CSVError.empty }
        return items
    }

    enum CSVError: LocalizedError {
        case empty, missingColumns, missingTitlePlatform, duplicateID, malformed
        var errorDescription: String? {
            switch self {
            case .empty: "CSV has no game rows"
            case .missingColumns: "CSV must include title and platform columns"
            case .missingTitlePlatform: "Each row needs a title and platform"
            case .duplicateID: "CSV contains duplicate ids"
            case .malformed: "Could not parse CSV"
            }
        }
    }

    private static func records(from text: String) throws -> [[String]] {
        var rows: [[String]] = []
        var row: [String] = []
        var field = ""
        var inQuotes = false
        var i = text.startIndex
        while i < text.endIndex {
            let ch = text[i]
            if inQuotes {
                if ch == "\"" {
                    let next = text.index(after: i)
                    if next < text.endIndex, text[next] == "\"" {
                        field.append("\"")
                        i = next
                    } else {
                        inQuotes = false
                    }
                } else {
                    field.append(ch)
                }
            } else {
                switch ch {
                case "\"":
                    inQuotes = true
                case ",":
                    row.append(field)
                    field = ""
                case "\n":
                    row.append(field)
                    rows.append(row)
                    row = []
                    field = ""
                case "\r":
                    break
                default:
                    field.append(ch)
                }
            }
            i = text.index(after: i)
        }
        if inQuotes { throw CSVError.malformed }
        if !field.isEmpty || !row.isEmpty {
            row.append(field)
            rows.append(row)
        }
        return rows
    }
}
