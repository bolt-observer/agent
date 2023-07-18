# Diamondhands plugin

## Introduction
This is a plugin to support [DiamondHands](https://swap.diamondhands.technology/).

Internally it will use [Boltz plugin](../boltz/). It is simple to disable diamonhands plugin in compile time, but
if you want to disable Boltz (but use Diamonhands) you will have to change init() in Boltz plugin to not register itself.
There is `--disableboltz` runtime option however (and also `--disablediamondhands`).
