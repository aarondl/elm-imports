module Main

import Browser
import Browser.Navigation as Nav
import Html exposing (..)
import Html.Attributes exposing (..)
import Routes exposing (Route)

init : Model -> Nav.Key -> Route
init model navKey =
    Nav.makeKey "hello"
