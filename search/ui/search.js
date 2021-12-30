(() => {

    // Search engine
    const STOP_WORDS = new Set("__KKR_STOP_WORDS__".split(' ')); // stop words set by kkr when generating a site
    const isStopWord = w => STOP_WORDS.has(w);

    var ACCENTS = {
        224: 'a', 225: 'a', 226: 'a', 227: 'a', 228: 'a', 229: 'a', 230: 'a',
        231: 'c', 232: 'e', 233: 'e', 234: 'e', 235: 'e', 236: 'i', 237: 'i',
        238: 'i', 239: 'i', 241: 'n', 242: 'o', 243: 'o', 244: 'o', 245: 'o',
        246: 'o', 339: 'o', 249: 'u', 250: 'u', 251: 'u', 252: 'u', 253: 'y',
        255: 'y', 8217: "'"
    };

    function normalizeWord(w) {
        let out = '';
        for (let i = 0; i < w.length; i++) {
            c = w.charCodeAt(i);
            if (c >= 768 && c <= 879) {
                continue; // skip composed accent
            }
            const s = ACCENTS[c] || String.fromCharCode(c);
            if ((i == 0 || i == w.length - 1) && s == "'") {
                continue; // exclude apostrophes at the beginning and at the end
            }
            out += s;
        }
        return out.toLowerCase();
    }

    function stem(w) {
        // Don't stem words that contain digits,
        // since this JS stemmer relies on digits
        // for internal work, for some reason.
        return /\d/.test(w) ? w : stemmer(w); // stemmer() comes from stemmer.min.js
    }

    function search(searchIndex, query) {
        const queryWords = (query.match(/[\p{L}\d'’]{1,}/gu) || []).map(normalizeWord);
        // const lastWord = queryWords.pop(); // XXX incomplete last word search disabled, see below.
        const words = queryWords.filter(w => !isStopWord(w)).map(stem);

        const found = {}; // maps words to documents and frequencies
        words.forEach(w => {
            if (searchIndex.words[w]) {
                found[w] = searchIndex.words[w];
            }
        });

        // Consider last word in query a prefix and find the correct
        // word that matches it among all indexed words.
        // XXX Don't need this with current version.
        /*
        if (lastWord) {
            const stemmedLastWord = stemmer(lastWord);
            for (let indexWord of Object.keys(searchIndex.words)) {
                if (indexWord[0] === lastWord[0]) {
                    if (indexWord.indexOf(lastWord) === 0 ||
                        indexWord.indexOf(stemmedLastWord) === 0) {
                        found[indexWord] = searchIndex.words[indexWord];
                        break;
                    }
                }
            }
        }
        */

        const matchesByDoc = {}; // maps docs to the number of matched query words
        Object.values(found).forEach(arr => {
            arr.forEach(dc => {
                const d = typeof dc === "number" ? dc : dc[0];
                matchesByDoc[d] = (matchesByDoc[d] || 0) + 1;
            });
        });
        // console.log('docs', matchesByDoc);

        /*
        const foundDocs = [];
        Object.keys(docs).forEach(id => {
            if (docs[id] >= words.length - 1) { // allow 1 missed word
                foundDocs.push(+id);
            }
        });
        console.log('foundDocs', foundDocs);
        */

        // Rank documents by word count.
        const numDocsFound = Object.keys(matchesByDoc).length;
        const numDocsTotal = searchIndex.docs.length;
        const ranksByDoc = {}; // maps docs to a calculated rank
        Object.keys(found).forEach(word => {
            const val = found[word];
            const numDocsWithWord = val.length;
            val.forEach(dc => {
                const d = typeof dc === "number" ? dc : dc[0];
                const freq = typeof dc === "number" ? 1 : dc[1];
                const idf = Math.log(numDocsTotal / numDocsWithWord);
                const rank = freq * idf;
                ranksByDoc[d] = (ranksByDoc[d] || 0) + rank; //(r * matchesByDoc[d]);
            });
        });
        // console.log('ranksByDoc', ranksByDoc);

        const rankDocPairs = [];
        Object.keys(ranksByDoc).forEach(k => {
            rankDocPairs.push([k, ranksByDoc[k]]);
        });

        rankDocPairs.sort((a, b) => b[1] - a[1]);
        // console.log('rankDocPairs sorted', rankDocPairs);
        return rankDocPairs.map(dr => searchIndex.docs[dr[0]])
            .map(d => ({
                title: d.t,
                url: d.u
            }));
    }

    // UI
    const SEARCH_INDEX_URL = "__KKR_SEARCH_INDEX_URL__";  // actual location set by kkr when generating a site

    let searcher;

    async function getSearcher() {
        if (searcher) return searcher;
        return fetch(SEARCH_INDEX_URL)
            .then(r => r.json())
            .then(json => {
                searcher = q => search(json, q);
                return searcher;
            });
        // TODO error reporting
    }

    function handleSearchParams() {
        const url = new URL(window.location);
        const query = url.searchParams.get('query') || "";
        const page = url.searchParams.get('page') || 1;
        const input = document.querySelector('input#kkr-search-input');
        input.value = query;
        doSearch(query, page);
    }

    function setSearchParams(query, page) {
        const url = new URL(window.location);
        url.searchParams.set('query', query || "");
        url.searchParams.set('page', page || 1)
        window.history.pushState({}, '', url);
        doSearch(url.searchParams.get('query'), page);
    }

    async function doSearch(query, page) {
        const container = document.querySelector('#kkr-search-results');
        container.classList.add('loading');
        getSearcher().then(searcher => displayResults(query ? searcher(query) : null, page));
    }

    function displayResults(results, page) {
        const container = document.querySelector('#kkr-search-results');
        container.classList.remove('loading');
        container.classList.remove('not-found');
        container.innerHTML = "";

        if (results == null) return;

        if (container.dataset.onlyUrls) {
            const re = new RegExp(container.dataset.onlyUrls);
            results = results.filter(r => re.test(r.url));
        }

        if (container.dataset.cleanTitle) {
            const re = new RegExp(container.dataset.cleanTitle);
            results.forEach(r => {
                r.title = r.title.replace(re, '');
            });
        }

        if (results.length == 0) {
            container.classList.add('not-found');
            container.textContent = container.dataset.notFound || 'Nothing found'
        }

        const resultsPerPage = parseInt(container.dataset.resultsPerPage) || 10;
        const pageCount = Math.ceil(results.length / resultsPerPage);
        const offset = (page - 1) * resultsPerPage;

        results = results.slice(offset, offset + resultsPerPage);

        results.forEach(r => {
            const result = document.importNode(document.querySelector("template#kkr-search-result-item").content, true);
            result.querySelectorAll(".kkr-search-result-title").forEach(e => { e.textContent = r.title });
            result.querySelectorAll(".kkr-search-result-url").forEach(e => { e.textContent = r.url });
            result.querySelectorAll("a.kkr-search-result-href").forEach(e => { e.href = r.url });
            container.appendChild(result);
        });

        if (pageCount > 1) {
            const pagination = document.importNode(document.querySelector("template#kkr-search-result-pagination").content, true);
            const pages = [];
            for (let i = 1; i <= pageCount; i++) {
                const pageItem = document.importNode(pagination.querySelector(".kkr-search-result-page-item"), true);
                if (i == page) pageItem.classList.add('active');
                const link = pageItem.querySelector(".kkr-search-result-page-link");
                link.textContent = i;
                link.addEventListener('click', handlePageClick)
                pages.push(pageItem);
            }
            const pageItem = pagination.querySelector('.kkr-search-result-page-item');
            pages.forEach(p => pageItem.parentNode.appendChild(p));
            pageItem.remove();
            container.appendChild(pagination);
        }
    }

    function handlePageClick(ev) {
        ev.preventDefault();
        const p = parseInt(ev.target.textContent);
        if (!p) return;
        setSearchParams((new URL(window.location)).searchParams.get('query'), p);
        document.querySelector('input#kkr-search-input').scrollIntoView({ behavior: "smooth", block: "start" })
    }

    function installSearchUI() {
        const input = document.querySelector('input#kkr-search-input');
        if (!input) return false;

        input.addEventListener('keydown', ev => {
            if (ev.key === "Enter") {
                setSearchParams(input.value, 1);
            }
        });

        const button = document.querySelector('#kkr-search-button');
        if (button) {
            button.addEventListener('click', () => {
                setSearchParams(input.value, 1);
            });
        }
        return true;
    }

    function init() {
        if (installSearchUI()) {
            handleSearchParams();
        }
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    } else {
        init();
    }

})();
