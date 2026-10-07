<?php
// Generates php_json_pretty.txt: run `php php_json_pretty.php > php_json_pretty.txt`.
// It needs only PHP, not the PHP CLI removed in 1.0.0.
// Each line: the JSON input, then what json_encode(json_decode($input), $flags) returns for it with
// JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE and with JSON_PRETTY_PRINT |
// JSON_UNESCAPED_SLASHES ("false" when json_decode gives null or json_encode fails), all JSON-quoted,
// separated by tabs. json_decode without $associative gives objects, as memry's agent files are read.
$cases = [
    '{}', '[]', '{"a":1}', '{"a":{},"b":[]}', '{"a":[{}],"b":[[]]}', '{"0":"a","1":"b"}', '[1,"a",null,true,false]',
    '{"x":1,"x":2}', '{"x":1,"y":2,"x":3}', '{"":1}', '{"a/b":"c/d"}', '{"é":"Diseño ✓"}', '"😀"', '"\\ud83d\\ude00"',
    '"\\u0000\\u001f\\u007f"', '"\\u2028\\u2029"', '"<>&\'"', '"\\"\\\\\\b\\f\\n\\r\\t"', '{"n":[1.0,2.50,-0,-0.0,1e2,1e17,0.00001]}',
    '{"big":12345678901234567890,"int":9223372036854775807}', '{"inf":1e400}', '{"\\u0000a":1}', 'null', '',
    ' { "a" : [ 1 , 2 ] , "b" : { "c" : { } } } ', '{"a":', '{"a":1} x', '{"mcpServers":{"other":{"command":"npx","args":["-y","other"],"env":{}}}}',
];
$quote = fn ($value) => json_encode($value, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
foreach ($cases as $case) {
    $decoded = json_decode($case);
    echo $quote($case);
    foreach ([JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES] as $flags) {
        $encoded = $decoded === null && $case !== 'null' ? false : json_encode($decoded, $flags);
        echo "\t", $encoded === false ? 'false' : $quote($encoded);
    }
    echo "\n";
}
