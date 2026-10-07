<?php
// Generates php_json_encode.txt: run `php php_json_encode.php > php_json_encode.txt`.
// Each line: the JSON input and what json_encode(json_decode($input, true), JSON_UNESCAPED_SLASHES)
// returns for it ("false" when it fails), both JSON-quoted, separated by a tab.
$cases = [
    '1', '-1', '0', '-0', '42', '9223372036854775807', '9223372036854775808', '-9223372036854775808',
    '-9223372036854775809', '12345678901234567890', '123456789012345678901234567890',
    '1.0', '100.0', '1.5', '-1.5', '-0.0', '0.1', '0.0001', '0.00001', '1e2', '1E2', '1e-2', '1.5e-7', '2.5E+3',
    '1e15', '1e16', '1e17', '1e18', '1.0e+17', '123456789012345680000', '1.5e300', '5e-324', '0.3333333333333333',
    '12345.678', '1e400', '-1e400', '1.7976931348623157e308', '4.9e-324', '2.2250738585072014e-308',
    '0.1e1', '10e-1', '123456789.123456789', '1e-4', '1.25e-4', '99999999999999999', '1e21', '1.23e22',
    '"abc"', '""', '"a/b"', '"a\\/b"', '"\\u00e9"', '"é"', '"😀"', '"\\ud83d\\ude00"', '"\\u0000"', '"\\u001f"', '"\\u007f"',
    '"<>&\'"', '"\\"\\\\\\b\\f\\n\\r\\t"', '"\\u2028\\u2029"',
    'true', 'false', 'null',
    '[]', '{}', '[1,"a",null,true]', '[[],{}]', '{"a":1}', '{"a":{},"b":[]}', '{"0":"a","1":"b"}', '{"1":"a"}',
    '{"1":"a","0":"b"}', '{"0":"a","2":"b"}', '{"-1":"a"}', '{"01":"a"}', '{"-0":"a"}', '{" 1":"a"}', '{"1.0":"a"}',
    '{"x":1,"x":2}', '{"x":1,"y":2,"x":3}', '{"0":"a","0":"b"}', '{"":1}', '{"a/b":"c/d"}', '{"é":"é"}',
    '{"9223372036854775807":1}', '{"9223372036854775808":1}', '{"0":1,"1":2,"x":3}', '[{"0":"a"},{"1":"b"}]',
    ' { "a" : [ 1 , 2 ] } ', '[1.0,2.50,-0]', '{"a":1e400}',
];
foreach ($cases as $case) {
    echo json_encode($case, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE), "\t";
    $encoded = json_encode(json_decode($case, true), JSON_UNESCAPED_SLASHES);
    echo $encoded === false ? 'false' : json_encode($encoded, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE), "\n";
}
