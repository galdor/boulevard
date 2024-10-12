<?php
$data = [
  "\$_SERVER" => $_SERVER,
  "request_body" => file_get_contents("php://input"),
];

header("Content-Type: application/json");
echo json_encode($data, JSON_PRETTY_PRINT), "\n";
?>
