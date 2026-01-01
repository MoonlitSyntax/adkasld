(() => {
    "use strict";

    const onReady = (fn) => {
        if (document.readyState === "loading") {
            document.addEventListener("DOMContentLoaded", fn, { once: true });
        } else {
            fn();
        }
    };
    const debounce = (fn, wait = 300) => {
        let t;
        return (...args) => {
            clearTimeout(t);
            t = setTimeout(() => fn(...args), wait);
        };
    };

    function initSearch() {
        const searchButton  = document.getElementById("search-button");
        const searchMask    = document.getElementById("algolia-search-mask");
        const searchModal   = document.getElementById("algolia-search-modal");
        const searchInput   = document.getElementById("algolia-search-input");
        const searchResults = document.getElementById("algolia-search-results");
        const searchSummary = document.getElementById("algolia-search-summary");

        const closeButton   = document.getElementById("close-search-modal-btn")
            || document.querySelector(".close-button")
            || null;

        if (!searchButton || !searchMask || !searchModal || !searchInput || !searchResults || !searchSummary) {
            return;
        }

        let searchData = null;
        let loading = false;

        const ensureData = async () => {
            if (searchData) return true;
            if (loading) return false; // simple lock
            loading = true;
            try {
                // Determine base URL if needed, but relative path usually works
                const res = await fetch("/search_index.json");
                if (!res.ok) throw new Error("fetch failed");
                searchData = await res.json();
                loading = false;
                return true;
            } catch (err) {
                console.error("Failed to load search index", err);
                loading = false;
                return false;
            }
        };

        const openModal = async () => {
            searchMask.classList.add("active");
            searchModal.classList.add("active");
            document.body.style.overflow = "hidden";
            setTimeout(() => searchInput.focus(), 200);
            await ensureData();
        };
        const closeModal = () => {
            searchMask.classList.remove("active");
            searchModal.classList.remove("active");
            document.body.style.overflow = "";
        };

        const renderResults = (hits) => {
            if (!hits || !hits.length) {
                searchResults.innerHTML = `
          <div style="text-align:center;padding:40px 0;">
             <p style="color:var(--muted);">什么都没有喵~</p>
          </div>`;
                searchSummary.style.display = "block";
                searchSummary.textContent = "共找到 0 条结果";
                return;
            }
            
            // Limit output
            const showHits = hits.slice(0, 50);

            searchResults.innerHTML = showHits.map(hit => `
        <div class="search-hit">
          <a href="/post/${hit.slug}" target="_self">
            <strong>${hit.title || hit.slug}</strong>
            <small>${hit.date} ${hit.tags ? "· " + hit.tags.join(" ") : ""}</small>
          </a>
        </div>`).join("");
            searchSummary.style.display = "block";
            searchSummary.textContent = `共找到 ${hits.length} 条结果`;
        };

        const doSearch = debounce((q) => {
            if (!q) {
                searchResults.innerHTML = "";
                searchSummary.style.display = "none";
                return;
            }
            if (!searchData) {
                 if (loading) {
                     searchResults.innerHTML = "<p>正在加载索引...</p>";
                 } else {
                     searchResults.innerHTML = "<p>加载索引失败，请刷新重试</p>";
                 }
                 return;
            }
            
            const lowerQ = q.toLowerCase();
            const hits = searchData.filter(item => {
                if (item.title && item.title.toLowerCase().includes(lowerQ)) return true;
                if (item.slug && item.slug.toLowerCase().includes(lowerQ)) return true;
                if (item.tags && item.tags.some(t => t.toLowerCase().includes(lowerQ))) return true;
                return false;
            });
            renderResults(hits);
        }, 200);

        const onOpen = (e) => { e.preventDefault(); openModal(); };
        searchButton.addEventListener("click", onOpen);
        searchButton.addEventListener("touchend", onOpen);

        if (closeButton) closeButton.addEventListener("click", closeModal);
        searchMask.addEventListener("click", (e) => { if (e.target === searchMask) closeModal(); });
        document.addEventListener("keydown", (e) => { if (e.key === "Escape" && searchModal.classList.contains("active")) closeModal(); });

        searchInput.addEventListener("input", (e) => doSearch(e.target.value.trim()));
    }

    function initCodeBlocks() {
        document.querySelectorAll(".c-code__btn--copy").forEach((btn) => {
            btn.addEventListener("click", () => {
                const container = btn.closest(".c-code");
                if (!container) return;
                const pre = container.querySelector("pre");
                if (!pre) return;

                const lines = pre.querySelectorAll("span.cl");
                const text = lines.length
                    ? [...lines].map(sp => sp.textContent.replace(/\r?\n$/, "")).join("\n")
                    : pre.textContent.trimEnd();

                navigator.clipboard.writeText(text).then(() => {
                    btn.innerHTML = `<i class="fas fa-check"></i>`;
                    setTimeout(() => (btn.innerHTML = `<i class="fas fa-copy"></i>`), 1500);
                }).catch(() => {
                    const sel = window.getSelection();
                    const range = document.createRange();
                    range.selectNodeContents(pre);
                    sel.removeAllRanges();
                    sel.addRange(range);
                    try { document.execCommand("copy"); } catch {}
                    sel.removeAllRanges();
                });
            });
        });

        document.querySelectorAll(".c-code__btn--fold").forEach((btn) => {
            btn.addEventListener("click", () => {
                const container = btn.closest(".c-code");
                if (!container) return;
                container.classList.toggle("c-code--folded");
                const icon = btn.querySelector("i");
                const folded = container.classList.contains("c-code--folded");
                if (icon) icon.className = folded ? "fas fa-chevron-down" : "fas fa-chevron-up";
            });
        });
    }

    function initAutoHideNav() {
        const nav = document.querySelector(".c-nav");
        if (!nav) return;
        let lastY = window.scrollY;
        window.addEventListener("scroll", () => {
            const y = window.scrollY;
            if (y > lastY && y > 50) nav.classList.add("nav-hidden");
            else nav.classList.remove("nav-hidden");
            lastY = y;
        }, { passive: true });
    }


    function initTOC() {

        const tocContainer = document.querySelector(".c-post__toc");
        const tocList = document.querySelector(".c-post__toc-list");
        const tocEmpty = tocContainer ? tocContainer.querySelector(".c-post__toc-empty") : null;
        const content = document.querySelector(".c-post__content");
        if (!tocContainer || !tocList || !content) return;

        const headings = [...content.querySelectorAll("h1, h2, h3")];
        if (!headings.length) {
            if (tocEmpty) tocEmpty.style.display = "flex";
            return;
        }
        if (tocEmpty) tocEmpty.style.display = "none";

        tocList.innerHTML = "";
        const h3GroupMap = new Map();
        let lastH2Id = "";

        headings.forEach((h) => {
            if (!h.id) h.id = h.textContent.trim().replace(/\s+/g, "-");
            const level = parseInt(h.tagName[1], 10) || 2;

            const li = document.createElement("li");
            li.className = `c-post__toc-item level-${level}`;

            const a = document.createElement("a");
            a.href = `#${h.id}`;
            a.textContent = h.textContent;
            li.appendChild(a);

            if (level === 3) {
                if (!h3GroupMap.has(lastH2Id)) h3GroupMap.set(lastH2Id, []);
                h3GroupMap.get(lastH2Id).push(li);
            } else {
                tocList.appendChild(li);
                if (level === 2) lastH2Id = h.id;
            }
        });

        const observer = new IntersectionObserver((entries) => {
            entries.forEach((entry) => {
                const id = entry.target.id;
                const link = tocList.querySelector(`a[href="#${id}"]`);
                if (!link) return;

                if (entry.isIntersecting && entry.target.tagName === "H2") {
                    tocList.querySelectorAll("a").forEach(a => a.classList.remove("active-h2"));
                    link.classList.add("active-h2");

                    tocList.querySelectorAll("li.level-3").forEach(li => li.remove());
                    if (h3GroupMap.has(id)) {
                        const after = link.parentElement;
                        const h3lis = h3GroupMap.get(id);
                        for (let i = h3lis.length - 1; i >= 0; i--) {
                            tocList.insertBefore(h3lis[i], after.nextSibling);
                        }
                    }
                    const cRect = tocContainer.getBoundingClientRect();
                    const lRect = link.getBoundingClientRect();
                    if (lRect.top < cRect.top || lRect.bottom > cRect.bottom) {
                        const offset = link.offsetTop - (tocContainer.clientHeight / 3);
                        tocContainer.scrollTo({ top: offset, behavior: "smooth" });
                    }
                }
            });
        }, { rootMargin: "0px 0px -60% 0px", threshold: 0.6 });

        headings.forEach(h => observer.observe(h));

        tocList.addEventListener("click", (e) => {
            const a = e.target.closest("a");
            if (!a) return;
            e.preventDefault();
            const id = a.getAttribute("href").slice(1);
            const target = document.getElementById(id);
            if (!target) return;
            const OFFSET = -60;
            const top = target.getBoundingClientRect().top + window.scrollY + OFFSET;
            window.scrollTo({ top, behavior: "smooth" });
        });
    }

    function initImages() {

        const processImageContainer = (container) => {
            const img = container.querySelector('.c-img__img');
            if (!img) return;
            const updateAspectRatio = () => {
                const width = img.naturalWidth;
                const height = img.naturalHeight;

                if (width > 0 && height > 0) {
                    const ratio = width / height;
                    container.style.setProperty('--aspect-ratio', ratio);
                }
                container.classList.add('is-loaded');
            };

            if (img.complete) {
                updateAspectRatio();
            } else {
                img.addEventListener('load', updateAspectRatio, { once: true });
                img.addEventListener('error', () => container.classList.add('is-error'), { once: true });
            }
        };

        document.querySelectorAll(".c-img").forEach(processImageContainer);

        const observer = new MutationObserver((mutations) => {
            for (const mutation of mutations) {
                if (mutation.type === 'childList') {
                    mutation.addedNodes.forEach(node => {
                        if (node.nodeType === 1) {
                            if (node.matches('.c-img')) {
                                processImageContainer(node);
                            } else {
                                node.querySelectorAll('.c-img').forEach(processImageContainer);
                            }
                        }
                    });
                }
            }
        });

        observer.observe(document.body, { childList: true, subtree: true });
        let lightbox = document.querySelector(".c-lightbox");
        if (!lightbox) {
            lightbox = document.createElement("div");
            lightbox.className = "c-lightbox";
            lightbox.innerHTML = '<img class="c-lightbox__img" alt=""/>';
            document.body.appendChild(lightbox);
        }

        const openLB = (src, alt) => {
            const imgElement = lightbox.querySelector(".c-lightbox__img");
            imgElement.src = src;
            imgElement.alt = alt || "";
            lightbox.classList.add("is-open");
            document.documentElement.style.overflow = "hidden";
        };

        const closeLB = () => {
            lightbox.classList.remove("is-open");
            document.documentElement.style.overflow = "";
        };

        lightbox.addEventListener("click", closeLB);
        window.addEventListener("keydown", (e) => { if (e.key === "Escape") closeLB(); });

        document.addEventListener("click", (e) => {
            const box = e.target.closest(".c-img");
            if (!box) return;

            const fullSrc = box.dataset.full || box.querySelector(".c-img__img")?.src;
            const altText = box.querySelector(".c-img__img")?.alt || "";

            if (fullSrc) {
                e.preventDefault();
                openLB(fullSrc, altText);
            }
        });
    }

    function initPagination() {
        document.addEventListener("submit", (e) => {
            const form = e.target.closest('form[data-pager]');
            if (!form) return;

            e.preventDefault();

            const input = form.querySelector('input[name="p"]');
            const raw = (input?.value || "").trim();
            let p = parseInt(raw.replace(/[^\d]/g, ""), 10);
            if (!Number.isFinite(p) || p < 1) p = 1;

            const total = parseInt(form.dataset.total || "0", 10);
            if (Number.isFinite(total) && total > 0) {
                p = Math.min(Math.max(p, 1), total);
            }

            const base = form.dataset.base || "/";
            const mode = (form.dataset.mode || "section").toLowerCase();

            let href;
            if (mode === "home") {
                href = p <= 1 ? "/" : `/page/${p}`;
            } else {
                href = p <= 1 ? base : `${base}/${p}`;
            }
            location.href = href;
        }, { passive: false });

        document.addEventListener("keydown", (e) => {
            if (e.key !== "Enter") return;
            const form = e.target && e.target.closest && e.target.closest('form[data-pager]');
            if (!form) return;
            form.requestSubmit ? form.requestSubmit() : form.dispatchEvent(new Event("submit", { cancelable: true }));
        }, { passive: true });
    }

    onReady(() => {
        initSearch();
        initCodeBlocks();
        initAutoHideNav();
        initTOC();
        initImages();
        initPagination();
    });
})();