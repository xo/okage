# make.py writes the test data of the package tinycss2 from the Python
# package tinycss2, which is the oracle. Run it from this folder with the
# checkout of tinycss2 at the commit of docs/upstream/tinycss2.tsv, and with
# webencodings installed:
#
#     python3 -I make.py path/to/tinycss2
#
# Each file holds one case on each line: the JSON of the input, a tab, and
# the JSON of the result of upstream. A lone surrogate in a result becomes
# U+FFFD, as in the port (D50).

import json
import random
import sys

sys.path.insert(0, sys.argv[1])

import tinycss2  # noqa: E402
from tinycss2 import ast  # noqa: E402
from tinycss2 import color3, color4, color5, nth  # noqa: E402
from webencodings import lookup  # noqa: E402


def numeric(t):
    return [t.representation, t.value,
            'integer' if t.int_value is not None else 'number']


def to_json(v):
    if v is None or isinstance(v, (str, int, float, bool)):
        return v
    if isinstance(v, (list, tuple)):
        return [to_json(x) for x in v]
    t = type(v)
    if t is ast.ParseError:
        return ['error', v.kind]
    if t is ast.Comment:
        return '/* … */'
    if t is ast.WhitespaceToken:
        return ' '
    if t is ast.LiteralToken:
        return v.value
    if t is ast.IdentToken:
        return ['ident', v.value]
    if t is ast.AtKeywordToken:
        return ['at-keyword', v.value]
    if t is ast.HashToken:
        return ['hash', v.value, 'id' if v.is_identifier else 'unrestricted']
    if t is ast.StringToken:
        return ['string', v.value]
    if t is ast.URLToken:
        return ['url', v.value]
    if t is ast.NumberToken:
        return ['number', *numeric(v)]
    if t is ast.PercentageToken:
        return ['percentage', *numeric(v)]
    if t is ast.DimensionToken:
        return ['dimension', *numeric(v), v.unit]
    if t is ast.UnicodeRangeToken:
        return ['unicode-range', v.start, v.end]
    if t is ast.CurlyBracketsBlock:
        return ['{}', *to_json(v.content)]
    if t is ast.SquareBracketsBlock:
        return ['[]', *to_json(v.content)]
    if t is ast.ParenthesesBlock:
        return ['()', *to_json(v.content)]
    if t is ast.FunctionBlock:
        return ['function', v.name, *to_json(v.arguments)]
    if t is ast.Declaration:
        return ['declaration', v.name, to_json(v.value), v.important]
    if t is ast.AtRule:
        return ['at-rule', v.at_keyword, to_json(v.prelude), to_json(v.content)]
    if t is ast.QualifiedRule:
        return ['qualified rule', to_json(v.prelude), to_json(v.content)]
    raise TypeError(t)


def positions(nodes):
    result = []
    for n in nodes:
        item = [n.type, n.source_line, n.source_column]
        for attr in ('content', 'arguments', 'prelude', 'value'):
            sub = getattr(n, attr, None)
            if isinstance(sub, list):
                item.append(positions(sub))
        result.append(item)
    return result


def fix(s):
    # A Go string cannot hold a lone surrogate.
    return ''.join('\ufffd' if 0xD800 <= ord(c) <= 0xDFFF else c for c in s)


def dumps(v):
    return fix(json.dumps(v, ensure_ascii=False))


def write(name, cases, function):
    with open(name + '.txt', 'w', encoding='utf-8') as f:
        for case in cases:
            try:
                result = function(case)
            except Exception as e:  # noqa: BLE001
                result = ['exception', type(e).__name__]
            f.write(json.dumps(case, ensure_ascii=False) + '\t' + dumps(result) + '\n')


HAND = [
    '', ' ', 'a', 'foo bar', '-', '--', '-a', '--a', '-1', '-.5', '+.5e-3',
    '\\', '\\\n', 'a\\', 'a\\\n', '\\41 b', '\\41\tb', '\\000041', '\\0',
    '\\110000', '\\FFFFFF', 'a\\:b', 'é', '\x00', 'a\rb\r\nc\fd',
    '12', '12.', '12.5', '.5', '1e3', '1E+3', '1e', '1e+', '1ex', '1e3px',
    '12%', '12.5%', '12px', '12PX', '1\\65 2', '34\\45 56a', '1-a', '1--',
    '99999999999999999999', '-99999999999999999999', '1e999', '1e-999',
    'u+1', 'U+0-7F', 'u+1?', 'u+??????', 'u+0-', 'U+FFFFFFF', 'u+a-f00000',
    'u+', 'U+g', '@', '@a', '@-', '@--', '@1', '@\\41', '#', '#a', '#1',
    '#-', '#--a', '#\\', '#\\\n', '#é', '{', '}', '[', ']', '(', ')',
    '{]', '(}', '[)', '{a{b{c}}}', 'f(', 'f()', 'f(a, b)', 'url(', 'url()',
    'url( )', 'url(a)', 'url( a )', 'url(a b)', 'url(a"b)', 'url(a\\)b)',
    'url(a(b)', 'url("a")', "url('a'", 'url(a', 'url(a ', 'url(\\', 'URL(x)',
    'uRl(x)', 'url(\x01)', 'url(a\\\nb)', 'url(a\\', '"', "'", '"a', '"a\\',
    '"a\\\nb"', '"a\nb"', "'a\\'b'", '"\\"', '"é"', '/*', '/* a', '/**/',
    '/* a */b', 'a/**/b', '<!--', '-->', '<!-- a -->', '||', '|=', '~=',
    '^=', '$=', '*=', '|', '~', '^', '$', '*', '!', '!important',
    'a:b', 'a:b;c:d', 'a:b !important', 'a:b!IMPORTANT;', 'a:b ! important',
    'a', 'a;', ':b', 'a b:c', 'a:{}', 'a:b {}', 'a:{} b', 'a: {} ',
    '@import "a.css";', '@media print { a { color: red } }', '@a', '@a;',
    '@a {', '@a b c', 'a{}', 'a {', 'a', 'a;b{}', '{}', '{} a', 'a{} b{}',
    'a{b:c;d:e}', 'a { b: c; @d e; f { g: h } }', 'a { & b { c: d } }',
    'foo#bar.baz { color: red } /**/ @media print{}',
    'color: #123; /**/ @top-left {} width:7px !important;',
    '<!-- a{} -->', 'a{}<!--b{}', ';;a:b;;', 'a:b;;c', '@a; b',
    'a:b }', 'a:b; } c:d', 'a:b {c} d', 'a:b; c {d:e}', 'a { b:c } d:e;',
    'calc(1px + 2%)', 'rgb(0, 0, 0)', 'width: calc(calc());',
    '\n  a\n  b\n', 'a\n\nb', 'a\n /* x\ny */ b', 'ab\ncd ef\n\n  gh',
    'a\\', '\\-', '-\\', '-\\\n', '--\\', '0\\65', '0e', '0E1', '0\\',
    'a(b(c(d)))', '((((', '))))', '[[a]]', 'a,b', 'a , b', 'n', '2n+1',
    'url(a)b', 'url(a)url(b)', '"a"b', '1a', '1-', '1_', 'é1', '1é',
    'a\x7fb', 'a\u2028b', 'a\U0001F600b', '\\1F600', 'U+?', 'u+-1',
]

ALPHABET = [
    'a', 'b', '-', '_', '\\', '\n', ' ', '\t', '"', "'", '(', ')', '[', ']',
    '{', '}', '/', '*', '!', '@', '#', '.', ',', ';', ':', '%', '+', '0',
    '1', '9', 'e', 'E', 'u', 'U', '?', '<', '>', '~', '|', '^', '$', '=',
    'url(', 'url( ', 'important', 'é', '\x00', '\r', '\f', 'x', 'n',
    'calc(', 'rgb(', 'deg', 'U+', '0.5', '1e3', '--', '-->', '<!--', '/*',
    '*/', '\\A ', '\\d800', 'px', '%', '\x01', '\U0001F600',
]


def random_cases(seed, count, max_len):
    rng = random.Random(seed)
    cases = []
    for _ in range(count):
        n = rng.randint(1, max_len)
        cases.append(''.join(rng.choice(ALPHABET) for _ in range(n)))
    return cases


CASES = HAND + random_cases(1, 300, 14)

SKIP = {'skip_comments': True, 'skip_whitespace': True}

write('component_value_list', CASES,
      lambda c: to_json(tinycss2.parse_component_value_list(c, skip_comments=True)))
write('component_value_list_comments', CASES,
      lambda c: to_json(tinycss2.parse_component_value_list(c)))
write('positions', CASES,
      lambda c: positions(tinycss2.parse_component_value_list(c)))
write('one_component_value', CASES,
      lambda c: to_json(tinycss2.parse_one_component_value(c, skip_comments=True)))
write('declaration_list', CASES,
      lambda c: to_json(tinycss2.parse_declaration_list(c, **SKIP)))
write('blocks_contents', CASES,
      lambda c: to_json(tinycss2.parse_blocks_contents(c, **SKIP)))
write('blocks_contents_positions', CASES,
      lambda c: positions(tinycss2.parse_blocks_contents(c)))
write('one_declaration', CASES,
      lambda c: to_json(tinycss2.parse_one_declaration(c, skip_comments=True)))
write('stylesheet', CASES,
      lambda c: to_json(tinycss2.parse_stylesheet(c, **SKIP)))
write('stylesheet_positions', CASES,
      lambda c: positions(tinycss2.parse_stylesheet(c)))
write('rule_list', CASES,
      lambda c: to_json(tinycss2.parse_rule_list(c, **SKIP)))
write('one_rule', CASES,
      lambda c: to_json(tinycss2.parse_one_rule(c, skip_comments=True)))
write('serialize', CASES,
      lambda c: tinycss2.serialize(tinycss2.parse_component_value_list(c)))
write('serialize_round_trip', CASES,
      lambda c: to_json(tinycss2.parse_component_value_list(
          tinycss2.serialize(tinycss2.parse_component_value_list(c, skip_comments=True)),
          skip_comments=True)))
write('serialize_rules', CASES,
      lambda c: tinycss2.serialize(tinycss2.parse_rule_list(c)))
write('serialize_blocks', CASES,
      lambda c: tinycss2.serialize(tinycss2.parse_blocks_contents(c)))

NTH = [
    '', ' ', 'n', '-n', '+n', '+ n', 'N', '2n', '2n+1', '2n +1', '2n+ 1',
    '2n + 1', '2n-1', '2n- 1', '2n -1', '2n - 1', '-2n+3', '+2n', '+ 2n',
    'odd', 'even', 'ODD', 'Even', ' odd ', '3', '+3', '-3', '0', '0n',
    '0n+0', 'n-1', 'n- 1', '-n-1', '-n- 1', '+n-1', 'n+-1', 'n-+1', '2n+-1',
    '2n-+1', '+', '+/**/', 'n+', 'n +', '-n-', '2n +', '2N+1', 'n /**/ + 1',
    '1.5n', '2.0n', '3.0', '3e1', 'n-1a', '2n-1a', '2n1', 'n1', 'odd 1',
    '-n- -1', 'n-01', '2n-01', '+n-2', '+N-2', '+n- 2', '+2', '+-2', '- 2',
    '-', 'n\\-1', '2n\\-1', '-n\\-1', '\\6e', '99999999999999999999n',
    'n+99999999999999999999', 'n-1\\A', '2n-1\\A', '+ odd', 'a', '/**/n',
]

write('nth', NTH, lambda c: nth.parse_nth(c))
write('nth_round_trip', NTH,
      lambda c: (lambda r: nth.parse_nth(nth.serialize_nth(*r)) if r else None)(nth.parse_nth(c)))


def number(value):
    if value is None:
        return 'none'
    value = round(value + 0.0000001, 6)
    return str(int(value) if value.is_integer() else value)


def build(color):
    if color is None:
        return None
    if isinstance(color, str):
        return color
    if isinstance(color, color3.RGBA):
        return 'rgba(' + ' '.join(number(c) for c in color) + ')'
    result = f'color({color.space}'
    for coordinate in color.coordinates:
        result += f' {number(coordinate)}'
    result += f' / {number(color.alpha)})'
    return result


def conversions(color):
    if color is None or isinstance(color, str):
        return None
    result = []
    for space in ('srgb', 'xyz', 'xyz-d50', 'xyz-d65', 'lab'):
        try:
            result.append(build(color.to(space)))
        except NotImplementedError:
            result.append(None)
    return result


COLORS = [
    '', 'red', 'RED', 'Red', 'transparent', 'currentcolor', 'currentColor',
    'CURRENTCOLOR', 'rebeccapurple', 'grey', 'aliceblue', 'notacolor',
    '#', '#0', '#00', '#000', '#0000', '#00000', '#000000', '#0000000',
    '#00000000', '#abc', '#ABCD', '#abcdef', '#ABCDEF12', '#ggg', '#12345g',
    '#abc\\A', '#abcdef\\A', 'rgb()', 'rgb(1,2,3)', 'rgb(1, 2, 3)',
    'rgb(1 2 3)', 'rgb(1,2,3,4)', 'rgb(1,2,3,0.5)', 'rgba(1,2,3)',
    'rgba(1,2,3,0.5)', 'rgba(1,2,3,50%)', 'rgb(1 2 3 / 0.5)',
    'rgb(1 2 3 / 50%)', 'rgb(1.5, 2, 3)', 'rgb(1%, 2%, 3%)', 'rgb(1%, 2, 3)',
    'rgb(-10%, 120%, 0%)', 'rgb(300, -5, 0)', 'rgb(none 2 3)',
    'rgb(none none none)', 'rgb(1,2)', 'rgb(1,2,3,4,5)', 'rgb(1 2 3 4)',
    'rgb(1 2 3 / none)', 'rgba(1, 2, 3, 2)', 'rgba(1, 2, 3, -1)',
    'RGB(1,2,3)', 'rgb(1,/**/2,3)', 'rgb( 1 , 2 , 3 )', 'rgb(1,2,3,)',
    'hsl(0, 100%, 50%)', 'hsl(120, 100%, 50%)', 'hsl(240 100% 50%)',
    'hsl(120deg 100% 50%)', 'hsl(0.5turn 100% 50%)', 'hsl(1rad 50% 50%)',
    'hsl(200grad 50% 50%)', 'hsl(-120 50% 50%)', 'hsl(480 50% 50%)',
    'hsl(none 50% 50%)', 'hsl(0 none 50%)', 'hsl(0 50 50)',
    'hsla(0, 100%, 50%, 0.5)', 'hsla(0, 100%, 50%)', 'hsl(0, 100%, 50%, 50%)',
    'hsl(0 100% 50% / 0.25)', 'hsl(0, 0%, 50%)', 'hsl(30DEG 50% 50%)',
    'hsl(0 50% 50% / 1 / 2)', 'hsl(1e3 50% 50%)', 'hsl(0.1 33.3% 66.7%)',
    'hwb(0 0% 0%)', 'hwb(120 30% 50%)', 'hwb(0 60% 60%)', 'hwb(0 50 50)',
    'hwb(0, 0%, 0%)', 'hwb(none 20% 30%)', 'hwb(90deg 10% 10% / 0.5)',
    'lab(50% 40 59.5)', 'lab(50 40 59.5)', 'lab(50% 40% 59.5%)',
    'lab(none 40 59.5)', 'lab(50, 40, 59.5)', 'lab(50 40 59.5 / 0.5)',
    'lch(50% 30 120)', 'lch(50 30% 120deg)', 'lch(50 30 none)',
    'lch(50, 30, 120)', 'lch(50 30 0.5turn / 25%)', 'oklab(50% 0.1 -0.1)',
    'oklab(0.5 10% -10%)', 'oklab(none 0.1 0.1)', 'oklch(50% 0.1 120)',
    'oklch(0.5 25% 3rad)', 'oklch(0.5 0.1 none / 0.5)', 'oklch(0.5, 0.1, 1)',
    'color(srgb 1 0 0)', 'color(srgb 100% 0% 0%)', 'color(srgb-linear 1 0 0)',
    'color(display-p3 1 0 0)', 'color(a98-rgb 1 0 0)',
    'color(prophoto-rgb 1 0 0)', 'color(rec2020 1 0 0)', 'color(xyz 1 0 0)',
    'color(xyz-d50 0.5 0.5 0.5)', 'color(xyz-d65 0.5 0.5 0.5 / 0.5)',
    'color(XYZ 1 0 0)', 'color(srgb none 0 0)', 'color(unknown 1 0 0)',
    'color()', 'color(srgb)', 'color(srgb 1 0)', 'color(srgb, 1, 0, 0)',
    'color(--custom 1 2 3)', 'color(--custom 1 2 3 4 5)',
    'color(--custom 10% none 3 / 0.5)', 'color(--custom 1 2 3 / none)',
    'color(-custom 1 2 3)', 'device-cmyk(0 0 0 1)', 'device-cmyk(0, 0, 0, 1)',
    'device-cmyk(0% 50% 100% 0%)', 'device-cmyk(2 -1 0 0)',
    'device-cmyk(0 0 0 1 / 0.5)', 'device-cmyk(0 0 0)',
    'device-cmyk(0%, 0%, 0%, 0%)', 'device-cmyk(0 0 0 1 / 2 3)',
    'light-dark(red, blue)', 'light-dark(red blue)', 'light-dark(red)',
    'light-dark(red, blue, green)', 'light-dark(currentcolor, blue)',
    'light-dark(light-dark(red, blue), green)', 'light-dark(nope, blue)',
    'foo(1, 2, 3)', '1', '1px', '"red"', 'red blue', '/**/red',
    'rgb(1 2 3/0.5)', 'rgb(1 2 3 /0.5)', 'hsl(0 50% 50%/.5)',
]

write('color3', COLORS,
      lambda c: build(color3.parse_color(c)))
write('color4', COLORS,
      lambda c: build(color4.parse_color(c)))
write('color5', COLORS,
      lambda c: build(color5.parse_color(c)))
write('color5_dark', COLORS,
      lambda c: build(color5.parse_color(c, ('dark',))))
write('color4_to', COLORS,
      lambda c: conversions(color4.parse_color(c)))
write('color3_tokens', COLORS,
      lambda c: build(color3.parse_color(tinycss2.parse_one_component_value(c))))

BYTES = [
    {'css_bytes': ''},
    {'css_bytes': 'a{}'},
    {'css_bytes': '\xef\xbb\xbfa{}'},
    {'css_bytes': '\xff\xfea\x00{\x00}\x00'},
    {'css_bytes': '\xfe\xff\x00a\x00{\x00}'},
    {'css_bytes': '\xff\xfea\x00{\x00}'},
    {'css_bytes': '\xff\xfe\x00\xd8a\x00'},
    {'css_bytes': '\xff\xfe\x3d\xd8\x00\xde'},
    {'css_bytes': '\xff\xfe\x00\xdca\x00'},
    {'css_bytes': '\xff\xfe\x3d\xd8'},
    {'css_bytes': '\xff\xfe\x3d\xd8a'},
    {'css_bytes': 'a\xe9{}'},
    {'css_bytes': 'a\xe9{}', 'protocol_encoding': 'latin1'},
    {'css_bytes': 'a\xe9{}', 'protocol_encoding': ' ISO-8859-1 '},
    {'css_bytes': 'a\x80\x81\x9f{}', 'protocol_encoding': 'windows-1252'},
    {'css_bytes': 'a\xe9{}', 'protocol_encoding': 'foo'},
    {'css_bytes': 'a\xe9{}', 'environment_encoding': 'latin1'},
    {'css_bytes': 'a\xe9{}', 'protocol_encoding': 'utf-8', 'environment_encoding': 'latin1'},
    {'css_bytes': '@charset "latin1";a\xe9{}'},
    {'css_bytes': '@charset "latin1"; a\xe9{}', 'environment_encoding': 'utf-8'},
    {'css_bytes': '@charset "utf-16";a{}'},
    {'css_bytes': '@charset "utf-16be";a{}'},
    {'css_bytes': '@charset "foo";a\xe9{}'},
    {'css_bytes': '@charset "latin1"a\xe9{}'},
    {'css_bytes': '@charset  "latin1";a\xe9{}'},
    {'css_bytes': "@charset 'latin1';a\xe9{}"},
    {'css_bytes': '@charset "x-user-defined";a\xe9{}'},
    {'css_bytes': '@charset "' + 'a' * 95 + '";'},
    {'css_bytes': '\xef\xbb\xbf@charset "latin1";a\xe9{}'},
    {'css_bytes': '\xc3\xa9\xc3'},
    {'css_bytes': '\xe2\x82'},
    {'css_bytes': '\xe2\x82a'},
    {'css_bytes': '\xed\xa0\x80'},
    {'css_bytes': '\xf0\x9f\x98'},
    {'css_bytes': '\xf4\x90\x80\x80'},
    {'css_bytes': '\xc0\xaf'},
    {'css_bytes': '\xe0\x80\xaf'},
    {'css_bytes': '\xff\xfe'},
    {'css_bytes': 'a\xe9{}', 'protocol_encoding': 'utf-16'},
    {'css_bytes': 'a\x00{\x00', 'protocol_encoding': 'utf-16le'},
]


def parse_bytes(kwargs):
    kwargs = dict(kwargs)
    kwargs['css_bytes'] = kwargs['css_bytes'].encode('latin1')
    if kwargs.get('environment_encoding'):
        kwargs['environment_encoding'] = lookup(kwargs['environment_encoding'])
    kwargs.update(SKIP)
    rules, encoding = tinycss2.parse_stylesheet_bytes(**kwargs)
    return [to_json(rules), encoding.name]


write('stylesheet_bytes', BYTES, parse_bytes)
