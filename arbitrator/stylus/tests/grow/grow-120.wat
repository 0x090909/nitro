;; Copyright 2023, Offchain Labs, Inc.
;; For license information, see https://github.com/0x090909/nitro/blob/master/LICENSE

(module
    (func (export "user_entrypoint") (param $args_len i32) (result i32)
        i32.const 0
    )
    (memory (export "memory") 120 120)
)
