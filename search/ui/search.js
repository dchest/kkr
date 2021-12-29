(() => {

    // Search engine
    const STOP_WORDS = {};
    [
        "all", "am", "an", "and", "any", "are", "aren't", "as", "at", "be",
        "because", "been", "before", "being", "below", "between", "both",
        "but", "by", "can't", "cannot", "could", "couldn't", "did", "didn't",
        "do", "does", "doesn't", "doing", "don't", "down", "for", "from",
        "further", "had", "hadn't", "has", "hasn't", "have", "haven't",
        "having", "he", "he'd", "he'll", "he's", "her", "here", "here's",
        "hers", "herself", "him", "himself", "his", "how", "how's", "i'd",
        "i'll", "i'm", "i've", "if", "in", "into", "is", "isn't", "it", "it's",
        "its", "itself", "let's", "me", "more", "most", "mustn't", "my",
        "myself", "no", "nor", "not", "of", "off", "on", "once", "only", "or",
        "other", "ought", "our", "ours ", "ourselves", "out", "over", "own",
        "same", "shan't", "she", "she'd", "she'll", "she's", "should",
        "shouldn't", "so", "some", "such", "than", "that", "that's", "the",
        "their", "theirs", "them", "themselves", "then", "there", "there's",
        "these", "they", "they'd", "they'll", "they're", "they've", "this",
        "those", "through", "to", "too", "under", "until", "up", "very", "was",
        "wasn't", "we", "we'd", "we'll", "we're", "we've", "were", "weren't",
        "what", "what's", "when", "when's", "where", "where's", "which",
        "while", "who", "who's", "whom", "why", "why's", "with", "won't",
        "would", "wouldn't", "you", "you'd", "you'll", "you're", "you've",
        "your", "yours", "yourself", "yourselves"
    ].forEach(w => { STOP_WORDS[w] = true });

    function isStopWord(w) { return !!STOP_WORDS[w]; }

    var ACCENTS = {
        224: 'a', 225: 'a', 226: 'a', 227: 'a', 228: 'a', 229: 'a', 230: 'a',
        231: 'c', 232: 'e', 233: 'e', 234: 'e', 235: 'e', 236: 'i', 237: 'i',
        238: 'i', 239: 'i', 241: 'n', 242: 'o', 243: 'o', 244: 'o', 245: 'o',
        246: 'o', 339: 'o', 249: 'u', 250: 'u', 251: 'u', 252: 'u', 253: 'y',
        255: 'y'
    };

    function removeAccents(w) {
        var out = '', rep;
        for (var i = 0; i < w.length; i++) {
            c = w.charCodeAt(i);
            if (c >= 768 && c <= 879) {
                continue; // skip composed accent
            }
            rep = ACCENTS[c];
            out += rep ? rep : w.charAt(i);
        }
        return out;
    }

    function search(searchIndex, query) {
        const queryWords = (removeAccents(query).match(/\w{1,}/g) || []).map(s => s.toLowerCase());
        // const lastWord = queryWords.pop(); // XXX incomplete last word search disabled, see below.
        const words = queryWords.filter(w => !isStopWord(w)).map(stemmer);
        const found = {};
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

        const docs = {};
        Object.values(found).forEach(arr => {
            arr.forEach(dc => {
                const d = typeof dc === "number" ? dc : dc[0];
                docs[d] = (docs[d] || 0) + 1;
            });
        });

        const foundDocs = [];
        Object.keys(docs).forEach(id => {
            if (docs[id] >= words.length - 1) { // allow 1 missed word
                foundDocs.push(+id);
            }
        })

        // Rank documents by word count.
        const ranksByDoc = {};
        Object.values(found).forEach(arr => {
            arr.forEach(dc => {
                const d = typeof dc === "number" ? dc : dc[0];
                if (foundDocs.includes(d)) {
                    const r = typeof dc === "number" ? 1 : dc[1];
                    ranksByDoc[d] = (ranksByDoc[d] || 0) + r;
                }
            });
        });

        const rankDocPairs = [];
        Object.keys(ranksByDoc).forEach(k => {
            rankDocPairs.push([k, ranksByDoc[k]]);
        });

        rankDocPairs.sort((a, b) => b[1] - a[1]);
        return rankDocPairs.map(dr => dr[0])
            .map(id => searchIndex.docs[id])
            .map(d => ({
                title: d.t,
                url: d.u
            }));
    }

    // UI
    const SEARCH_INDEX_URL = "__KKR_SEARCH_INDEX_URL__";  // actual location URL by kkr when generating a site

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
        console.log('Doing search', query, page);
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