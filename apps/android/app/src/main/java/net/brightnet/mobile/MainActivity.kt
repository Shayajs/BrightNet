package net.brightnet.mobile

import android.Manifest
import android.annotation.SuppressLint
import android.app.DownloadManager
import android.content.Context
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.webkit.CookieManager
import android.webkit.SslErrorHandler
import android.webkit.URLUtil
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.net.http.SslError
import android.widget.Toast
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.isVisible
import net.brightnet.mobile.databinding.ActivityMainBinding

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private var coreUrl: String? = null
    private var pendingDownload: Triple<String, String?, String?>? = null

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)
        ViewCompat.setOnApplyWindowInsetsListener(binding.root) { view, insets ->
            val bars = insets.getInsets(WindowInsetsCompat.Type.systemBars())
            view.setPadding(bars.left, bars.top, bars.right, bars.bottom)
            insets
        }

        val web = binding.web
        web.setOnApplyWindowInsetsListener { _, insets -> insets }
        web.settings.javaScriptEnabled = true
        web.settings.domStorageEnabled = true
        web.settings.cacheMode = WebSettings.LOAD_DEFAULT
        web.settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
        web.settings.allowFileAccess = false
        web.settings.allowContentAccess = false
        web.settings.setSupportZoom(false)
        web.settings.userAgentString = web.settings.userAgentString + " BrightNetAndroid/1"
        web.setDownloadListener { url, _, contentDisposition, mime, _ -> save(url, contentDisposition, mime) }
        web.addJavascriptInterface(Bridge(), "BrightNetAndroid")
        web.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                val target = request.url?.toString() ?: return true
                val page = coreUrl ?: return true
                if (!CoreAddress.sameOrigin(page, target)) {
                    toast(getString(R.string.blocked))
                    return true
                }
                return false
            }

            override fun onPageFinished(view: WebView?, url: String?) {
                binding.progress.isVisible = false
            }

            override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                if (request.isForMainFrame) showFailure(getString(R.string.unreachable))
            }

            override fun onReceivedHttpError(view: WebView, request: WebResourceRequest, errorResponse: WebResourceResponse) {
                if (request.isForMainFrame && errorResponse.statusCode >= 500) showFailure(getString(R.string.unreachable))
            }

            override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
                handler.cancel()
                showFailure(getString(R.string.bad_certificate))
            }
        }
        web.webChromeClient = object : android.webkit.WebChromeClient() {
            override fun onProgressChanged(view: WebView?, newProgress: Int) {
                binding.progress.isVisible = newProgress in 1..99
                binding.progress.progress = newProgress
            }
        }
        CookieManager.getInstance().setAcceptCookie(true)
        CookieManager.getInstance().setAcceptThirdPartyCookies(web, false)

        binding.open.setOnClickListener { openTyped() }
        binding.retry.setOnClickListener { coreUrl?.let { open(it) } }
        binding.change.setOnClickListener { showGate() }

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                when {
                    binding.failure.isVisible -> showGate()
                    binding.webPane.isVisible -> web.evaluateJavascript(
                        "(function(){return window.brightnetBack?window.brightnetBack():false})()"
                    ) { value ->
                        if (value == "true") return@evaluateJavascript
                        if (web.canGoBack()) web.goBack() else showGate()
                    }
                    else -> {
                        isEnabled = false
                        onBackPressedDispatcher.onBackPressed()
                    }
                }
            }
        })

        val saved = getSharedPreferences(PREFS, MODE_PRIVATE).getString(KEY, null)
        val normalized = saved?.let(CoreAddress::normalize)
        if (normalized == null) showGate() else open(normalized)
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode != REQUEST_SAVE) return
        val pending = pendingDownload
        pendingDownload = null
        if (grantResults.firstOrNull() == PackageManager.PERMISSION_GRANTED && pending != null) {
            save(pending.first, pending.second, pending.third)
        }
    }

    override fun onPause() {
        CookieManager.getInstance().flush()
        super.onPause()
    }

    private fun openTyped() {
        val normalized = CoreAddress.normalize(binding.address.text?.toString().orEmpty())
        if (normalized == null) {
            binding.gateError.text = getString(R.string.bad_address)
            binding.gateError.isVisible = true
            return
        }
        binding.gateError.isVisible = false
        open(normalized)
    }

    private fun open(url: String) {
        coreUrl = url
        getSharedPreferences(PREFS, MODE_PRIVATE).edit().putString(KEY, url).apply()
        binding.address.setText(url)
        showWeb()
        binding.web.loadUrl(url)
    }

    private fun save(url: String, disposition: String?, mime: String?) {
        val page = coreUrl ?: return
        if (!CoreAddress.sameOrigin(page, url)) {
            toast(getString(R.string.blocked))
            return
        }
        if (Build.VERSION.SDK_INT <= 28 && checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) != PackageManager.PERMISSION_GRANTED) {
            pendingDownload = Triple(url, disposition, mime)
            requestPermissions(arrayOf(Manifest.permission.WRITE_EXTERNAL_STORAGE), REQUEST_SAVE)
            return
        }
        val name = URLUtil.guessFileName(url, disposition, mime)
        val request = DownloadManager.Request(Uri.parse(url))
            .setMimeType(mime)
            .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
            .setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, name)
            .setTitle(name)
        CookieManager.getInstance().getCookie(url)?.let { request.addRequestHeader("Cookie", it) }
        val manager = getSystemService(Context.DOWNLOAD_SERVICE) as DownloadManager
        manager.enqueue(request)
        toast(getString(R.string.downloading))
    }

    private fun showGate() {
        binding.gate.isVisible = true
        binding.webPane.isVisible = false
        binding.failure.isVisible = false
    }

    private fun showWeb() {
        binding.gate.isVisible = false
        binding.webPane.isVisible = true
        binding.failure.isVisible = false
    }

    private fun showFailure(message: String) {
        binding.failureText.text = message
        binding.gate.isVisible = false
        binding.webPane.isVisible = false
        binding.failure.isVisible = true
    }

    private fun toast(text: String) {
        Toast.makeText(this, text, Toast.LENGTH_SHORT).show()
    }

    private inner class Bridge {
        @android.webkit.JavascriptInterface
        fun changeCore() {
            runOnUiThread { showGate() }
        }
    }

    companion object {
        private const val PREFS = "brightnet"
        private const val KEY = "core_url"
        private const val REQUEST_SAVE = 1
    }
}
