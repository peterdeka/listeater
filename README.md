# listeater [![Build Status](https://travis-ci.org/peterdeka/listeater.svg?branch=master)](https://travis-ci.org/peterdeka/listeater) [![Coverage Status](https://coveralls.io/repos/github/peterdeka/listeater/badge.svg?branch=master)](https://coveralls.io/github/peterdeka/listeater?branch=master)
A simple crawler aimed at eating paginated lists of elements.

A lot of crawlers exist outside, however i needed a simple and configurable crawler to do the hard job of crawling different list types. 

As lists are always the same but list elements are quite different, the element crawl is delegated to your custom function. Every element crawling is run in its own goroutine.

Have fun.