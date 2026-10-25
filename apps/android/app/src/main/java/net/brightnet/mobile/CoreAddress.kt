package net.brightnet.mobile

import java.net.URI
import java.net.URISyntaxException

object CoreAddress {
    fun normalize(raw: String): String? {
        val trimmed = raw.trim()
        if (trimmed.isEmpty() || trimmed.any { it.isWhitespace() }) return null
        val withScheme = if ("://" in trimmed) trimmed else "http://$trimmed"
        val uri = try {
            URI(withScheme)
        } catch (_: URISyntaxException) {
            return null
        }
        val scheme = uri.scheme?.lowercase() ?: return null
        if (scheme != "http" && scheme != "https") return null
        if (uri.rawUserInfo != null) return null
        val host = uri.host ?: return null
        if (host.isBlank()) return null
        if (!uri.rawQuery.isNullOrEmpty() || uri.rawFragment != null) return null
        val path = uri.rawPath ?: ""
        if (path.isNotEmpty() && path != "/") return null
        val port = uri.port
        val auth = if (':' in host) {
            if (port == -1) "[$host]" else "[$host]:$port"
        } else if (port == -1) {
            host
        } else {
            "$host:$port"
        }
        return "$scheme://$auth"
    }

    fun sameOrigin(page: String, target: String): Boolean {
        val left = runCatching { URI(page) }.getOrNull() ?: return false
        val right = runCatching { URI(target) }.getOrNull() ?: return false
        if (!left.scheme.equals(right.scheme, ignoreCase = true)) return false
        if (!left.host.equals(right.host, ignoreCase = true)) return false
        return portOf(left) == portOf(right)
    }

    private fun portOf(uri: URI): Int {
        if (uri.port != -1) return uri.port
        return if (uri.scheme.equals("https", ignoreCase = true)) 443 else 80
    }
}
