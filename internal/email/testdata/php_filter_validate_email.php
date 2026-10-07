<?php
// Generates php_filter_validate_email.txt: run `php php_filter_validate_email.php > php_filter_validate_email.txt`.
// It needs only PHP, not the PHP CLI removed in 1.0.0.
$c = [
 'ana@example.com','not-an-email',"ana@example.co\xc3m",'','@','ana@','@example.com','ana@example','ana@localhost',
 'a.b@example.com','a..b@example.com','.ab@example.com','ab.@example.com','a+b@example.com','a_b-c@sub.example.co.uk',
 "a!#$%&'*+/=?^_`{|}~-@example.com",'a"b@example.com','"a b"@example.com','"a\\"b"@example.com','"a\\\\b"@example.com',
 '""@example.com','"a"."b"@example.com','"a".b@example.com','a(b)@example.com','a,b@example.com','a:b@example.com',
 'a;b@example.com','a<b@example.com','a>b@example.com','a[b@example.com','a\\b@example.com','a@b@example.com',
 'ana@example.com.','ana@.example.com','ana@example..com','ana@-example.com','ana@example-.com','ana@ex--ample.com',
 'ana@example.c','ana@example.c0m','ana@example.0com','ana@example.123','ana@123.com','ana@1.2.3.4','ana@xn--bcher-kva.ch',
 'ana@example.xn--p1ai','ana@example.xn--','ana@xn--.com','ana@ex_ample.com','ana@example.com-','ana@example.com-a',
 'ana@[1.2.3.4]','ana@[256.1.1.1]','ana@[01.2.3.4]','ana@[ipv6:::1]','ana@[ipv6:1:2:3:4:5:6:7:8]','ana@[ipv6:1:2:3:4:5:6:7]',
 'ana@[ipv6:1:2:3::5:6:7]','ana@[ipv6:1:2:3:4::5:6:7]','ana@[ipv6:1::2]','ana@[ipv6::ffff:1.2.3.4]','ana@[ipv6:1:2:3:4:5:6:1.2.3.4]',
 'ana@[ipv6:1:2:3:4:5::1.2.3.4]','ana@[ipv6:1:2::3:1.2.3.4]','ana@[ipv6:1:2:3:4::1.2.3.4]','ana@[ipv6:1::2:3:4:1.2.3.4]','ana@[1.2.3]',
 'ana@[ipv6:1:2:3:4:5:6:7:8:9]','ana@[ipv6:12345::1]','ana@[ipv6:g::1]','ana@[::1]','ana@[ipv6:::]',"ana@example.com\n","an\na@example.com",
 "ana@exa\nmple.com","ana@example.com\x00", "\x00ana@example.com",'ana@ex ample.com','a na@example.com','ANA@EXAMPLE.COM',
 str_repeat('a',64).'@example.com',str_repeat('a',65).'@example.com','ana@'.str_repeat('a',63).'.com','ana@'.str_repeat('a',64).'.com',
 'ana@example.'.str_repeat('a',63),'ana@example.'.str_repeat('a',64),
 str_repeat('a',64).'@'.str_repeat(str_repeat('b',60).'.',3).'com', str_repeat('a',64).'@'.str_repeat(str_repeat('b',60).'.',4).'com',
 str_repeat('a',64).'@'.str_repeat(str_repeat('b',61).'.',4).'com',
 '"'.str_repeat('a',62).'"@example.com','"'.str_repeat('a',63).'"@example.com','"'.str_repeat('a',64).'"@example.com','"'.str_repeat('a',65).'"@example.com',
 str_repeat('\\a',32).'@example.com','"'.str_repeat('\\a',32).'"@example.com','"'.str_repeat('\\a',33).'"@example.com','"'.str_repeat('\\a',65).'"@example.com',
 '"'.str_repeat('a"."',40).'"@example.com', str_repeat('"a".',30).'"a"@example.com',str_repeat('"a".',33).'"a"@example.com',
 '"a\\'."\xc3".'"@example.com',"\"a\x01b\"@example.com","\"a\x7fb\"@example.com","a\x7fb@example.com",'"a@b"@example.com',str_repeat('"a@',30).'x@example.com',
 'a@'.str_repeat('a.',126).'com','a@'.str_repeat('a.',127).'com','a@'.str_repeat('a.',150).'com',
 'a@b.c-d','a@b.c--d','a@b.xn--c-d','a@b.cd-','a@0.c','a@b.c.d.e.f','a@b-.c','a@b_c.d',
 'ana@exámple.com','ánà@example.com','ana@example.com ','  ana@example.com',
];
// boundary lengths for 320
$c[] = str_repeat('a',64).'@'.str_repeat(str_repeat('b',63).'.',3).str_repeat('c',59);
$c[] = str_repeat('a',64).'@'.str_repeat(str_repeat('b',63).'.',3).str_repeat('c',60);
$c[] = str_repeat('a',64).'@'.str_repeat(str_repeat('b',63).'.',3).str_repeat('c',63);
$c[] = str_repeat('a',64).'@'.str_repeat(str_repeat('b',63).'.',4).str_repeat('c',2);
$c[] = 'a@'.str_repeat(str_repeat('b',63).'.',3).str_repeat('c',60);
// random fuzz from alphabet
mt_srand(42);
$alpha = ['a','b','Z','0','.','-','_','@','"','\\','[',']',':','+',' ',"\t",'x','n','-','.','1','2','5','f','i','p','v','6'];
for ($i = 0; $i < 1500; $i++) { $n = mt_rand(1, 18); $s=''; for ($j=0;$j<$n;$j++) $s .= $alpha[mt_rand(0,count($alpha)-1)]; $c[] = $s; }
for ($i = 0; $i < 800; $i++) { $parts=['a','ab','xn--a','a-b','0','"a"','"a b"','a\\b','1','255','ipv6:','[',']','::','f:',':1','.','-']; $s=''; $n=mt_rand(1,6); for($j=0;$j<$n;$j++) $s.=$parts[mt_rand(0,count($parts)-1)]; $s.='@'; $n=mt_rand(1,7); for($j=0;$j<$n;$j++) $s.=$parts[mt_rand(0,count($parts)-1)]; $c[]=$s; }
for ($i = 0; $i < 2500; $i++) {
  $atoms=['a','b0','x-y','"q"','"a b"','"\\""','"x\\y"','+','a_b','"','\\a','..','-'];
  $loc=[]; $n=mt_rand(1,4); for($j=0;$j<$n;$j++) $loc[]=$atoms[mt_rand(0,count($atoms)-1)];
  $local=implode(mt_rand(0,5)?'.':'', $loc);
  if (mt_rand(0,3)===0) {
    $g=[]; $n=mt_rand(1,9); for($j=0;$j<$n;$j++) $g[] = mt_rand(0,6)? dechex(mt_rand(0, mt_rand(0,1)?15:65535)) : '';
    $v6=implode(':',$g); if (mt_rand(0,1)) { $k=mt_rand(1,5); $a=[]; for($j=0;$j<$k;$j++) $a[]=dechex(mt_rand(0,65535)); $b=[]; $k=mt_rand(0,4); for($j=0;$j<$k;$j++) $b[]=dechex(mt_rand(0,65535)); $v6=implode(':',$a).'::'.implode(':',$b); } if(mt_rand(0,1)) $v6 .= ':'.mt_rand(0,300).'.'.mt_rand(0,255).'.'.mt_rand(0,255).'.'.mt_rand(0,255);
    $dom = mt_rand(0,4) ? '[ipv6:'.$v6.']' : '['.mt_rand(0,300).'.'.mt_rand(0,255).'.'.mt_rand(0,99).'.'.mt_rand(0,9).']';
  } else {
    $labels=['a','ex-ample','xn--p1ai','0','1a','a1','-a','a-','b--c','xn--','com','c0m','x_y']; $d=[]; $n=mt_rand(1,4); for($j=0;$j<$n;$j++) $d[]=$labels[mt_rand(0,count($labels)-1)];
    $dom=implode('.',$d);
  }
  $c[] = $local.'@'.$dom;
}
$seen=[];
function goQuote(string $s): string {
  $out = '';
  foreach (str_split($s) as $b) {
    $o = ord($b);
    $out .= match (true) {
      $b === '"' => '\\"', $b === '\\' => '\\\\', $b === "\n" => '\\n', $b === "\t" => '\\t', $b === "\r" => '\\r',
      $o >= 0x20 && $o < 0x7F => $b,
      default => sprintf('\\x%02x', $o),
    };
  }
  return '"'.$out.'"';
}
echo "# Each line: 1 or 0 (whether PHP 8.4's filter_var(strtolower(trim(\$email)), FILTER_VALIDATE_EMAIL)\n";
echo "# accepts it), a space, and the email as a Go-quoted string. Generated by php_filter_validate_email.php.\n";
echo "# Left out: addresses PHP rejects only because PCRE hits pcre.backtrack_limit, such as seven or more\n";
echo "# quoted parts in a row (\"a\".\"a\".\"a\".\"a\".\"a\".\"a\".\"a\".\"a\"@example.com); Normalize accepts those.\n";
foreach ($c as $s) {
  if (isset($seen[$s]) || str_contains($s, str_repeat('"a".', 7)) || str_contains($s, str_repeat('a"."', 7))) continue;
  $seen[$s] = 1;
  echo filter_var(strtolower(trim($s)), FILTER_VALIDATE_EMAIL) === false ? 0 : 1, ' ', goQuote($s), "\n";
}
