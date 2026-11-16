# make.py writes the test data of the package cssselect2 from the Python
# package cssselect2, which is the oracle. Run it from this folder with the
# checkouts of cssselect2 and tinycss2 at the commits of
# docs/upstream/cssselect2.tsv and docs/upstream/tinycss2.tsv, and with
# webencodings installed:
#
#     python3 -I make.py path/to/cssselect2 path/to/tinycss2
#
# Each file holds one case on each line: the JSON of the input, a tab, and
# the JSON of the result of upstream. The documents content.xhtml, ids.html
# and shakespeare.html are the test documents of upstream, byte for byte,
# with the license LICENSE-web-platform-tests.

import json
import sys
import xml.etree.ElementTree as etree  # noqa: N813

sys.path.insert(0, sys.argv[2])
sys.path.insert(0, sys.argv[1])

from cssselect2 import ElementWrapper, Matcher, SelectorError, compile_selector_list  # noqa: E402
from tests.w3_selectors import invalid_selectors, valid_selectors  # noqa: E402

IDS_ROOT = etree.parse('ids.html')
SHAKESPEARE_BODY = ElementWrapper.from_xml_root(
    etree.parse('shakespeare.html').find('.//{http://www.w3.org/1999/xhtml}body'))


def get_test_document():
    document = etree.parse('content.xhtml')
    parent = document.find(".//*[@id='root']")
    # Setup namespace tests
    for id in ('any-namespace', 'no-namespace'):
        div = etree.SubElement(parent, '{http://www.w3.org/1999/xhtml}div')
        div.set('id', id)
        div1 = etree.SubElement(div, '{http://www.w3.org/1999/xhtml}div')
        div1.set('id', id + '-div1')
        div2 = etree.SubElement(div, '{http://www.w3.org/1999/xhtml}div')
        div2.set('id', id + '-div2')
        div3 = etree.SubElement(div, 'div')
        div3.set('id', id + '-div3')
        div4 = etree.SubElement(div, '{http://www.example.org/ns}div')
        div4.set('id', id + '-div4')
    return document


TEST_DOCUMENT = get_test_document()


def write(name, cases, function):
    with open(name + '.txt', 'w', encoding='utf-8') as f:
        for case in cases:
            try:
                result = function(case)
            except SelectorError:
                result = 'SelectorError'
            except Exception as e:  # noqa: BLE001
                result = ['exception', type(e).__name__]
            f.write(json.dumps(case, ensure_ascii=False) + '\t' +
                    json.dumps(result, ensure_ascii=False) + '\n')


def query_ids(root, selector):
    selectors = compile_selector_list(selector)
    return [element.etree_element.get('id', 'nil') for element in root.query_all(*selectors)]


SELECT = [
    '*', 'div', 'div div', 'div, div div', 'div , div div', 'a[name]',
    'a[rel]', 'a[rel="tag"]', 'a[href*="localhost"]', 'a[href*=""]',
    'a[href^="http"]', 'a[href^="http:"]', 'a[href^=""]', 'a[href$="org"]',
    'a[href$=""]', 'div[foobar~="bc"]', 'div[foobar~="cde"]',
    '[foobar~="ab bc"]', '[foobar~=""]', '[foobar~=" \t"]',
    'div[foobar~="cd"]', 'a[rel="tAg"]', 'a[rel="tAg" s]', 'a[rel="tAg" i]',
    'a[href*="localHOST"]', 'a[href*="localHOST" s]',
    'a[href*="localHOST" i]', 'a[href^="hTtp"]', 'a[href^="hTtp" s]',
    'a[href^="hTtp" i]', 'a[href$="Org"]', 'a[href$="Org" S]',
    'a[href$="Org" I]', 'div[foobar~="BC"]', 'div[foobar~="BC" s]',
    'div[foobar~="BC" i]', '*[lang|="En"]', '[lang|="En-us"]',
    '*[lang|="en"]', '[lang|="en-US"]', '*[lang|="e"]', ':lang(EN)',
    '*:lang(en-US)', ':lang(En)', ':lang(e)', ':lang("en-US")',
    ':lang("*-US")', ':lang(\\*-US)', ':lang(en /* English */, fr /* French */)',
    'li:nth-child(3)', 'li:nth-child(10)', 'li:nth-child(2n)',
    'li:nth-child(even)', 'li:nth-child(+2n+0)', 'li:nth-child(2n+1)',
    'li:nth-child(odd)', 'li:nth-child(2n+4)', 'li:nth-child(3n+1)',
    'p > input:nth-child(2n of p input[type=checkbox])',
    'li:nth-last-child(1)', 'li:nth-last-child(0)', 'li:nth-last-child(2n+2)',
    'li:nth-last-child(even)', 'li:nth-last-child(2n+4)',
    ':nth-last-child(1 of [type=checkbox])', 'ol:first-of-type',
    'ol:nth-child(1)', 'ol:nth-of-type(2)', ':nth-of-type(1 of .e)',
    'ol:nth-last-of-type(2)', ':nth-last-of-type(1 of .e)',
    'span:only-child', 'div:only-child', 'div *:only-child',
    'p *:only-of-type', 'p:only-of-type', 'a:empty', 'a:EMpty', 'li:empty',
    ':root', 'html:root', 'li:root', '* :root', '.a', '.b', '*.a', 'ol.a',
    '.c', '*.c', 'ol *.c', 'ol li.c', 'li ~ li.c', 'ol > li.c', '#first-li',
    'li#first-li', '*#first-li', 'li div', 'li > div', 'div > div',
    'div>.c', 'div > .c', 'div + div', 'a ~ a', 'a[rel="tag"] ~ a',
    'ol#first-ol li:last-child', 'ol#first-ol *:last-child',
    '#outer-div:first-child', '#outer-div :first-child', 'a[href]',
    ':not(*)', 'a:not([href])', 'ol :Not([class])',
    'li:not(:nth-child(odd), #second-li)', 'li:not(li)', ':is(*)',
    ':is(div)', ':is(div, fieldset)', ':is(:::wrong)',
    ':is(div, :::wrong, fieldset)', 'div :is(div, div)', 'li:is(.c)',
    'input:is([type="text"])', 'div:is(:not(#outer-div))',
    'div:is(div::before)', ':where(*)', ':where(div)',
    ':where(div, fieldset)', ':where(:::wrong)',
    ':where(div, :::wrong, fieldset)', 'div :where(div, div)',
    'li:where(.c)', 'input:where([type="text"])',
    'div:where(:not(#outer-div))', 'div:where(div::before)', ':has(:hover)',
    'p:has(input)', 'p:has(fieldset input)', 'p:has(> fieldset)',
    'ol:has(> div)', 'ol:has(input, li)', 'ol:has(input, fieldset)',
    'ol:has(+ p)', 'ol:has(~ ol)', 'ol:has(>a, ~ ol)', 'ol:has(a,ol,  li  )',
    'ol:has(*)', 'ol:has(:not(li))', 'ol:has( > :not( li ))',
    'ol:has(:not(li, div))', r'di\a0 v', r'div\[', r'[h\a0 ref]', r'[h\]ref]',
    ':link', ':any-link', ':local-link', ':visited', ':hover', ':active',
    ':focus', ':target', ':host', ':enabled', ':disabled', ':checked',
    'a:not([href]), div div', 'a:not([href]) /* test */, div div',
    'a:not([href]), /* test */ div div', '/* test */a:not([href]),div div',
    'a:not([href]) , div div/* test */',
    '/* test */a:not([href]), /* test */ div div',
    '/* test */a:not([href])/* test */,div  div',
    '/* test */ a:not([href]), div/* test */ div',
    'a:not([href]) /* test */,/* test */div  div',
    'DIV', 'a[NAme]', 'HTML :link',
    # More cases of the port.
    '', ' ', 'div,', ',div', 'div ,, p', '::before', 'div::before',
    'div::after', 'div:before', 'p::first-line', 'p::marker', 'p::foo',
    'p::-webkit-foo', ':lang()', ':lang(1)', ':lang(en fr)', ':lang(en [x])',
    ':lang(en, "fr", de)', '[*|href]', '[|href]', '[href i]', '[href s]',
    '[href x]', '[href=a b]', '[href="a" i x]', '[]', '[=a]', '[*=a]',
    '[href~="a b"]', '[href|=""]', ':nth-child()', ':nth-child(n of)',
    ':nth-child(2n of li, div)', ':nth-child(odd of *)',
    ':nth-last-child(1 of :not(*))', ':nth-of-type(n)', ':nth-of-type(-n+2)',
    ':nth-last-of-type(n+2 of li)', ':nth-child(0n+1)', ':nth-child(-n+3)',
    ':nth-child(-2n+5)', ':nth-child(99999999999999999999n)',
    ':nth-child(-99999999999999999999n+3)', ':nth-foo(1)', ':foo(1)',
    ':has()', ':has(::before)', ':has(:::wrong)', ':has(+ *)',
    ':has(~ *)', ':has(> * > *)', ':not()', ':not(::before)',
    ':not(:::wrong)', ':is()', ':where()', ':scope', ':first-child',
    ':last-child', ':only-child', ':only-of-type', ':first-of-type',
    ':last-of-type', ':empty', '* > *', '* + *', '* ~ *', '* *',
    '*|*', '|*', 'ns|div', '*|div', '|div', 'a|', '*|', 'div.', 'div#',
    '#1a', '.1a', 'div >', 'div +', '> div', 'div > > p', 'div ~ + p',
    'div:not(.a .b)', ':not(div p)', ':is(li > div)', ':is(ol li) div',
    'li:has(div) > div', ':root > body', 'html > body > div',
    '[id]:not([id=""])', '[class~=c]', 'a[rel~="nofollow"]',
    '[foobar|="ab"]', 'div:where(#outer-div)', 'div#outer-div:is(#outer-div)',
    'p:only-child', 'input:checked', 'input:enabled', 'input:disabled',
    'fieldset :disabled', 'option:checked', ':link:not(a)',
]


def select_both(selector):
    return [query_ids(ElementWrapper.from_xml_root(IDS_ROOT), selector),
            query_ids(ElementWrapper.from_html_root(IDS_ROOT), selector)]


write('select', SELECT, select_both)


def specificity(selector):
    return [[list(s.specificity), s.pseudo_element, s.never_matches]
            for s in compile_selector_list(selector)]


write('specificity', SELECT, specificity)

W3 = [test for test in valid_selectors
      if not set(test.get('exclude', ())) & {'document', 'xhtml'}]
write('w3_valid', [test['selector'] for test in W3],
      lambda s: [element.id for element in
                 ElementWrapper.from_xml_root(TEST_DOCUMENT).query_all(*compile_selector_list(s))])
write('w3_invalid', [test['selector'] for test in invalid_selectors],
      lambda s: len(compile_selector_list(s)))

SHAKESPEARE = [
    '*', 'div:only-child', 'div:nth-child(even)', 'div:nth-child(2n)',
    'div:nth-child(odd)', 'div:nth-child(2n+1)', 'div:nth-child(n)',
    'div:last-child', 'div:first-child', 'div > div', 'div + div',
    'div ~ div', 'body', 'body div', 'div', 'div div', 'div div div',
    'div, div, div', 'div, a, span', '.dialog', 'div.dialog', 'div .dialog',
    'div.character, div.dialog', 'div.direction.dialog',
    'div.dialog.direction', 'div.dialog.scene', 'div.scene.scene',
    'div.scene .scene', 'div.direction .dialog ', 'div .dialog .direction',
    'div.dialog .dialog .direction', '#speech5', 'div#speech5',
    'div #speech5', 'div.scene div.dialog', 'div#scene1 div.dialog div',
    '#scene1 #speech1', 'div[class]', 'div[class=dialog]',
    'div[class^=dia]', 'div[class$=log]', 'div[class*=sce]',
    'div[class|=dialog]', 'div[class~=dialog]', ':lang(en)', ':lang(fr)',
    'div:has(> div.dialog)', ':nth-last-of-type(3)',
]
write('shakespeare', SHAKESPEARE,
      lambda s: sum(1 for _ in SHAKESPEARE_BODY.query_all(*compile_selector_list(s))))

MATCHER = [
    '*', 'div', 'DIV', 'li', '.c', '.a.b', '#first-li', 'li#first-li',
    'ol > li', 'ol li.c', '[lang]', '[lang|="en"]', ':lang(en)', 'a[href]',
    '|li', ':first-child', 'li:nth-child(2n)', 'div::before', 'li::marker',
    ':hover', ':is(li, div)', ':where(li)', 'p:has(input)', '#outer-div *',
    'html|*',
]


def matcher_cases(mode):
    matcher = Matcher()
    for i, selector in enumerate(MATCHER):
        for compiled in compile_selector_list(selector, {'html': 'http://www.w3.org/1999/xhtml'}):
            matcher.add_selector(compiled, i)
    wrap = ElementWrapper.from_html_root if mode == 'html' else ElementWrapper.from_xml_root
    result = []
    for element in wrap(IDS_ROOT).iter_subtree():
        result.append([element.etree_element.get('id', 'nil'), [
            [list(m[0]), m[1], m[2], m[3]] for m in matcher.match(element)]])
    return result


write('matcher', ['html', 'xml'], matcher_cases)
