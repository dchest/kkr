Stemmer package for Go
======================

Stemmer package provides an interface for stemmers and includes English,
German and Dutch stemmers as sub-packages:

 - `porter2` sub-package implements English (Porter2) stemmer as described in
   <http://snowball.tartarus.org/algorithms/english/stemmer.html>

 - `german` sub-package implements German stemmer as described in
   <http://snowball.tartarus.org/algorithms/german/stemmer.html>

 - `dutch` sub-package implements Dutch stemmer as described in
   <http://snowball.tartarus.org/algorithms/dutch/stemmer.html>


Installation
-------------

English stemmer:

    go get github.com/dchest/stemmer/porter2

German stemmer:

    go get github.com/dchest/stemmer/german

Dutch stemmer:

    go get github.com/dchest/stemmer/dutch

This will also install the top-level `stemmer` package.

Example
-------

    import (
        "github.com/dchest/stemmer/porter2"
        "github.com/dchest/stemmer/german"
        "github.com/dchest/stemmer/dutch"
    )

    // English.
    eng := porter2.Stemmer
    eng.Stem("delicious")   // => delici
    eng.Stem("deliciously") // => delici

    // German.
    ger := german.Stemmer
    ger.Stem("abhängen")   // => abhang
    ger.Stem("abhängiger") // => abhang

    // Dutch.
    dt := dutch.Stemmer
    dt.Stem("lichamelijke") // => licham
    dt.Stem("opglimpende")  // => opglimp

Tests
-----

Included `test_output.txt` and `test_voc.txt` are from the referenced original
implementations, used only when running tests with `go test`.


License
-------

2-clause BSD-like (see LICENSE and AUTHORS files).
