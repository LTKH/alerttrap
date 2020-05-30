#!/usr/bin/env node

'use strict';

const fs = require('fs')
const path = require('path')
const { basename } = path
const { dirname } = path
const { resolve } = path
const chalk = require('chalk')
const jsdom = require('jsdom')
const { JSDOM } = jsdom;
const beautify   = require('js-beautify').html
const jsbOptions = {
  indent_size: 2,
  indent_inner_html: true,
  unformatted: [''],
  content_unformatted: ['textarea'],
  extra_liners: ['']
}

const injectSVG = (data, selectors, includes) => {
  let dom
  let images
  const fullHtmlDocument = data.includes('<html') && data.includes('</html>')
  // check if full html file
  if (fullHtmlDocument) {
    dom = new jsdom.JSDOM(data)
    images = dom.window.document.querySelectorAll(selectors)
  } else {
    dom = JSDOM.fragment(`<dom>${data}</dom>`)
    images = dom.querySelectorAll(selectors)
  }

  if (!Array.isArray(Array.from(images)) || !Array.from(images).length) {
    console.log(chalk.red('There are no svg files inside.'))
    return data
  } else {
    images.forEach(image => {
      if (image.src.match(/.*\.svg$/)) {
        console.log(resolve(includes, image.src))
        const svg = new jsdom.JSDOM(fs.readFileSync(resolve(includes, image.src), 'ascii').toString(), 'image/svg+xml').window.document.querySelector('svg')
        const { width } = image
        const { height } = image
        if (height) {
          svg.setAttribute('height', height)
        }
        console.log(chalk.green(`${chalk.underline.bold(basename(image.src))} has been injected`))

        if (width) {
          svg.setAttribute('width', width)
        }
        svg.setAttribute('class', image.className)
        const vdom = new jsdom.JSDOM()
        image.parentNode.insertBefore(vdom.window.document.createTextNode('\n'), image.nextSibling)
        image.parentNode.insertBefore(vdom.window.document.createTextNode('\n'), image)
        image.parentNode.insertBefore(svg, image.nextSibling)
        image.remove()
      }
    })

    return fullHtmlDocument ? dom.serialize() : dom.firstChild.innerHTML
  }
}

const toFile = (file, selectors, includes) => {
  fs.readFile(file, { encoding: 'utf8' }, (err, data) => {
    console.log(resolve(file))
    if (err) {
      throw (err)
    }
    const incl = includes ? includes : dirname(file)
    const dom = injectSVG(data, selectors, incl)
    if (typeof dom !== "undefined") {
      fs.writeFile(file, beautify(dom, jsbOptions), err => {
        if (err) {
          throw (err)
        }
      })
    }
  })
}

module.exports = injectSVG
module.exports.toFile = toFile
