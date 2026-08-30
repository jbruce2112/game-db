import Foundation

enum APIError: LocalizedError {
    case unauthorized
    case server(String)
    case badURL

    var errorDescription: String? {
        switch self {
        case .unauthorized: "Unauthorized"
        case .server(let s): s
        case .badURL: "Invalid server URL"
        }
    }
}

struct SyncResponse: Codable {
    var cursor: Int64
    var changes: [LibraryItem]
}

final class APIClient {
    private var baseURL: URL?
    private var token: String?
    private let session: URLSession = {
        let c = URLSessionConfiguration.default
        c.timeoutIntervalForRequest = 20
        return URLSession(configuration: c)
    }()

    func configure(baseURL: URL, token: String?) {
        self.baseURL = baseURL
        self.token = token
    }

    func login(baseURL: URL, password: String) async throws -> String {
        self.baseURL = baseURL
        let body = try JSONEncoder().encode(["password": password])
        var req = URLRequest(url: baseURL.appending(path: "v1/auth/login"))
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = body
        let (data, resp) = try await session.data(for: req)
        try throwIfNeeded(resp, data: data)
        let parsed = try JSONDecoder().decode(TokenResponse.self, from: data)
        self.token = parsed.token
        return parsed.token
    }

    func logout() async throws {
        var req = try request(path: "/v1/auth/logout", method: "POST")
        req.httpBody = Data("{}".utf8)
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let (data, resp) = try await session.data(for: req)
        if let http = resp as? HTTPURLResponse, http.statusCode == 204 { return }
        try throwIfNeeded(resp, data: data)
    }

    func me() async throws -> MeResponse {
        let (data, resp) = try await session.data(for: try request(path: "/v1/auth/me", method: "GET"))
        try throwIfNeeded(resp, data: data)
        return try JSONDecoder().decode(MeResponse.self, from: data)
    }

    func importCSV(_ data: Data) async throws -> Int {
        var req = try request(path: "/v1/library/import", method: "POST")
        req.setValue("text/csv", forHTTPHeaderField: "Content-Type")
        req.httpBody = data
        let (body, resp) = try await session.data(for: req)
        try throwIfNeeded(resp, data: body)
        return try JSONDecoder().decode(ImportResponse.self, from: body).imported
    }

    func libraryItems() async throws -> [LibraryItem] {
        let (data, resp) = try await session.data(for: try request(path: "/v1/library", method: "GET"))
        try throwIfNeeded(resp, data: data)
        return try JSONDecoder().decode(LibraryEnvelope.self, from: data).items
    }

    func sync(cursor: Int64, changes: [LibraryItem]) async throws -> SyncResponse {
        struct Body: Codable { var cursor: Int64; var changes: [LibraryItem] }
        var req = try request(path: "/v1/sync", method: "POST")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONEncoder().encode(Body(cursor: cursor, changes: changes))
        let (data, resp) = try await session.data(for: req)
        try throwIfNeeded(resp, data: data)
        return try JSONDecoder().decode(SyncResponse.self, from: data)
    }

    func searchBarcode(_ q: String) async throws -> BarcodeSearch {
        var comps = URLComponents(url: try url("/v1/search/barcode"), resolvingAgainstBaseURL: false)!
        comps.queryItems = [URLQueryItem(name: "q", value: q)]
        let req = try request(url: comps.url!, method: "GET")
        let (data, resp) = try await session.data(for: req)
        try throwIfNeeded(resp, data: data)
        return try JSONDecoder().decode(BarcodeSearch.self, from: data)
    }

    func search(q: String) async throws -> [SearchGame] {
        var comps = URLComponents(url: try url("/v1/search/games"), resolvingAgainstBaseURL: false)!
        comps.queryItems = [URLQueryItem(name: "q", value: q)]
        let req = try request(url: comps.url!, method: "GET")
        let (data, resp) = try await session.data(for: req)
        try throwIfNeeded(resp, data: data)
        return try JSONDecoder().decode(SearchEnvelope.self, from: data).games
    }

    func createFromIGDB(gameId: Int64, platformId: Int64?, region: String?, completeness: String, barcode: String? = nil) async throws -> LibraryItem {
        struct Body: Codable {
            var igdbGameId: Int64
            var igdbPlatformId: Int64?
            var region: String?
            var completeness: String
            var barcode: String?
            enum CodingKeys: String, CodingKey {
                case igdbGameId = "igdb_game_id"
                case igdbPlatformId = "igdb_platform_id"
                case region, completeness, barcode
            }
        }
        var req = try request(path: "/v1/library", method: "POST")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONEncoder().encode(Body(
            igdbGameId: gameId, igdbPlatformId: platformId, region: region, completeness: completeness, barcode: barcode
        ))
        let (data, resp) = try await session.data(for: req)
        try throwIfNeeded(resp, data: data)
        return try JSONDecoder().decode(LibraryItem.self, from: data)
    }

    func cover(id: String) async throws -> Data {
        let req = try request(path: "/v1/covers/\(id)", method: "GET")
        let (data, resp) = try await session.data(for: req)
        try throwIfNeeded(resp, data: data)
        return data
    }

    private func request(path: String, method: String) throws -> URLRequest {
        try request(url: try url(path), method: method)
    }

    private func request(url: URL, method: String) throws -> URLRequest {
        var req = URLRequest(url: url)
        req.httpMethod = method
        if let token {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return req
    }

    private func url(_ path: String) throws -> URL {
        guard let baseURL else { throw APIError.badURL }
        let trimmed = path.hasPrefix("/") ? String(path.dropFirst()) : path
        return baseURL.appending(path: trimmed)
    }

    private func throwIfNeeded(_ resp: URLResponse, data: Data) throws {
        guard let http = resp as? HTTPURLResponse else { return }
        if http.statusCode == 401 { throw APIError.unauthorized }
        if (200..<300).contains(http.statusCode) { return }
        if let obj = try? JSONDecoder().decode(ErrorBody.self, from: data) {
            throw APIError.server(obj.error)
        }
        throw APIError.server("HTTP \(http.statusCode)")
    }
}

private struct TokenResponse: Codable { var token: String }
struct MeResponse: Codable {
    var igdbConfigured: Bool
    var pricechartingConfigured: Bool
    enum CodingKeys: String, CodingKey {
        case igdbConfigured = "igdb_configured"
        case pricechartingConfigured = "pricecharting_configured"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        igdbConfigured = try c.decodeIfPresent(Bool.self, forKey: .igdbConfigured) ?? false
        pricechartingConfigured = try c.decodeIfPresent(Bool.self, forKey: .pricechartingConfigured) ?? false
    }
}
private struct SearchEnvelope: Codable { var games: [SearchGame] }
private struct LibraryEnvelope: Codable { var items: [LibraryItem] }
private struct ImportResponse: Codable { var imported: Int }
private struct ErrorBody: Codable { var error: String }
