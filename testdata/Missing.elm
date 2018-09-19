module Main

import Browser

init : Api.Data.What
init =
    Api.Auth.makeKey "hello"
