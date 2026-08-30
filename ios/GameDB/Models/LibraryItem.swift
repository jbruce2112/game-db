import Foundation
import GRDB

struct LibraryItem: Identifiable, Equatable, Hashable {
    var id: String
    var title: String
    var platform: String
    var igdbPlatformId: Int64?
    var region: String?
    var completeness: String
    var notes: String
    var igdbGameId: Int64?
    var coverId: String?
    var barcode: String?
    var createdAt: String
    var updatedAt: String
    var deletedAt: String?
    var syncSeq: Int64
    var dirty: Bool
    var value: PriceQuote?

    static func newLocal(title: String, platform: String, region: String?, completeness: String, notes: String, barcode: String? = nil) -> LibraryItem {
        let now = Self.now()
        return LibraryItem(
            id: UUID().uuidString.lowercased(),
            title: title,
            platform: platform,
            igdbPlatformId: nil,
            region: region,
            completeness: completeness,
            notes: notes,
            igdbGameId: nil,
            coverId: nil,
            barcode: barcode,
            createdAt: now,
            updatedAt: now,
            deletedAt: nil,
            syncSeq: 0,
            dirty: true,
            value: nil
        )
    }

    static func now() -> String {
        RFC3339.string(Date())
    }

    func touching() -> LibraryItem {
        var copy = self
        copy.updatedAt = Self.now()
        copy.dirty = true
        return copy
    }
}

extension LibraryItem: Codable {
    enum CodingKeys: String, CodingKey {
        case id, title, platform, region, completeness, notes
        case igdbPlatformId = "igdb_platform_id"
        case igdbGameId = "igdb_game_id"
        case coverId = "cover_id"
        case barcode
        case createdAt = "created_at"
        case updatedAt = "updated_at"
        case deletedAt = "deleted_at"
        case syncSeq = "sync_seq"
        case value
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        title = try c.decode(String.self, forKey: .title)
        platform = try c.decode(String.self, forKey: .platform)
        igdbPlatformId = try c.decodeIfPresent(Int64.self, forKey: .igdbPlatformId)
        region = try c.decodeIfPresent(String.self, forKey: .region)
        completeness = try c.decodeIfPresent(String.self, forKey: .completeness) ?? "unknown"
        notes = try c.decodeIfPresent(String.self, forKey: .notes) ?? ""
        igdbGameId = try c.decodeIfPresent(Int64.self, forKey: .igdbGameId)
        coverId = try c.decodeIfPresent(String.self, forKey: .coverId)
        if let raw = try c.decodeIfPresent(String.self, forKey: .barcode), !raw.isEmpty {
            barcode = raw
        } else {
            barcode = nil
        }
        createdAt = try c.decode(String.self, forKey: .createdAt)
        updatedAt = try c.decode(String.self, forKey: .updatedAt)
        if let deleted = try c.decodeIfPresent(String.self, forKey: .deletedAt), !deleted.isEmpty {
            deletedAt = deleted
        } else {
            deletedAt = nil
        }
        syncSeq = try c.decodeIfPresent(Int64.self, forKey: .syncSeq) ?? 0
        dirty = false
        value = try c.decodeIfPresent(PriceQuote.self, forKey: .value)
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(id, forKey: .id)
        try c.encode(title, forKey: .title)
        try c.encode(platform, forKey: .platform)
        try c.encodeIfPresent(igdbPlatformId, forKey: .igdbPlatformId)
        try c.encodeIfPresent(region, forKey: .region)
        try c.encode(completeness, forKey: .completeness)
        try c.encode(notes, forKey: .notes)
        try c.encodeIfPresent(igdbGameId, forKey: .igdbGameId)
        try c.encodeIfPresent(coverId, forKey: .coverId)
        if let barcode {
            try c.encode(barcode, forKey: .barcode)
        } else {
            try c.encodeNil(forKey: .barcode)
        }
        try c.encode(createdAt, forKey: .createdAt)
        try c.encode(updatedAt, forKey: .updatedAt)
        try c.encodeIfPresent(deletedAt, forKey: .deletedAt)
        try c.encode(syncSeq, forKey: .syncSeq)
    }
}

extension LibraryItem: FetchableRecord, PersistableRecord {
    static let databaseTableName = "library_items"

    init(row: Row) {
        id = row["id"]
        title = row["title"]
        platform = row["platform"]
        igdbPlatformId = row["igdb_platform_id"]
        region = row["region"]
        completeness = row["completeness"]
        notes = row["notes"]
        igdbGameId = row["igdb_game_id"]
        coverId = row["cover_id"]
        barcode = row["barcode"]
        createdAt = row["created_at"]
        updatedAt = row["updated_at"]
        deletedAt = row["deleted_at"]
        syncSeq = row["sync_seq"]
        let flag: Int64 = row["dirty"]
        dirty = flag != 0
        value = nil
    }

    func encode(to container: inout PersistenceContainer) {
        container["id"] = id
        container["title"] = title
        container["platform"] = platform
        container["igdb_platform_id"] = igdbPlatformId
        container["region"] = region
        container["completeness"] = completeness
        container["notes"] = notes
        container["igdb_game_id"] = igdbGameId
        container["cover_id"] = coverId
        container["barcode"] = barcode
        container["created_at"] = createdAt
        container["updated_at"] = updatedAt
        container["deleted_at"] = deletedAt
        container["sync_seq"] = syncSeq
        container["dirty"] = dirty ? 1 : 0
    }
}

struct SearchGame: Codable, Identifiable {
    var id: Int64 { igdbId }
    var igdbId: Int64
    var name: String
    var summary: String?
    var coverUrl: String?
    var firstReleaseDate: String?
    var platforms: [SearchPlatform] = []

    enum CodingKeys: String, CodingKey {
        case igdbId = "igdb_id"
        case name, summary
        case coverUrl = "cover_url"
        case firstReleaseDate = "first_release_date"
        case platforms
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        igdbId = try c.decode(Int64.self, forKey: .igdbId)
        name = try c.decode(String.self, forKey: .name)
        summary = try c.decodeIfPresent(String.self, forKey: .summary)
        coverUrl = try c.decodeIfPresent(String.self, forKey: .coverUrl)
        firstReleaseDate = try c.decodeIfPresent(String.self, forKey: .firstReleaseDate)
        platforms = try c.decodeIfPresent([SearchPlatform].self, forKey: .platforms) ?? []
    }
}

struct SearchPlatform: Codable, Identifiable, Hashable {
    var id: Int64
    var name: String
}

struct BarcodeSearch: Codable {
    var barcode: String
    var productTitle: String?
    var query: String?
    var source: String?
    var platformHint: String?
    var platform: String?
    var lookupError: String?
    var games: [SearchGame]
    var owned: [OwnedCopy]

    enum CodingKeys: String, CodingKey {
        case barcode, query, source, games, owned
        case productTitle = "product_title"
        case platformHint = "platform_hint"
        case platform
        case lookupError = "lookup_error"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        barcode = try c.decode(String.self, forKey: .barcode)
        productTitle = try c.decodeIfPresent(String.self, forKey: .productTitle)
        query = try c.decodeIfPresent(String.self, forKey: .query)
        source = try c.decodeIfPresent(String.self, forKey: .source)
        platformHint = try c.decodeIfPresent(String.self, forKey: .platformHint)
        platform = try c.decodeIfPresent(String.self, forKey: .platform)
        lookupError = try c.decodeIfPresent(String.self, forKey: .lookupError)
        games = try c.decodeIfPresent([SearchGame].self, forKey: .games) ?? []
        owned = try c.decodeIfPresent([OwnedCopy].self, forKey: .owned) ?? []
    }
}

struct OwnedCopy: Codable, Identifiable {
    var id: String
    var title: String
    var platform: String
}

enum RFC3339 {
    static func string(_ date: Date) -> String {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        f.timeZone = TimeZone(secondsFromGMT: 0)
        return f.string(from: date)
    }
}
