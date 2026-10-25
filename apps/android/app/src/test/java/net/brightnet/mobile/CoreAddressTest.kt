package net.brightnet.mobile

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Test

class CoreAddressTest {
    @Test
    fun addsHttpAndDropsTheSlash() {
        assertEquals("http://192.168.1.3:1717", CoreAddress.normalize("192.168.1.3:1717"))
        assertEquals("http://192.168.1.3:1717", CoreAddress.normalize("http://192.168.1.3:1717/"))
    }

    @Test
    fun keepsHttps() {
        assertEquals("https://core.local", CoreAddress.normalize("https://core.local"))
    }

    @Test
    fun rejectsSecretsAndOtherSchemes() {
        assertNull(CoreAddress.normalize(""))
        assertNull(CoreAddress.normalize("javascript:alert(1)"))
        assertNull(CoreAddress.normalize("http://admin:secret@192.168.1.3:1717"))
        assertNull(CoreAddress.normalize("ftp://192.168.1.3"))
        assertNull(CoreAddress.normalize("http://192.168.1.3:1717/admin"))
        assertNull(CoreAddress.normalize("http://192.168.1.3:1717?x=1"))
    }

    @Test
    fun sameOriginIgnoresThePath() {
        assertEquals(true, CoreAddress.sameOrigin("http://192.168.1.3:1717", "http://192.168.1.3:1717/api/v1/downloads/brightnet-android.apk"))
        assertFalse(CoreAddress.sameOrigin("http://192.168.1.3:1717", "http://192.168.1.3:8787/"))
        assertFalse(CoreAddress.sameOrigin("http://192.168.1.3:1717", "https://192.168.1.3:1717"))
    }
}
